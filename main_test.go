package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectHardware(t *testing.T) {
	tests := []struct {
		name        string
		hwContent   string
		expected    *Hardware
		expectError bool
	}{
		{
			name:      "Alpha hardware",
			hwContent: "alpha\n",
			expected:  &HWAlpha,
		},
		{
			name:      "Beta hardware",
			hwContent: "beta",
			expected:  &HWBeta,
		},
		{
			name:      "PCIe hardware",
			hwContent: "pcie\n",
			expected:  &HWPcie,
		},
		{
			name:        "Unknown hardware",
			hwContent:   "unknown",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "hw")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpFile.Name())
			
			if _, err := tmpFile.Write([]byte(tt.hwContent)); err != nil {
				t.Fatal(err)
			}
			tmpFile.Close()
			
			result, err := detectHardwareFromFile(tmpFile.Name())
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result.Version != tt.expected.Version {
					t.Errorf("Expected version %s, got %s", tt.expected.Version, result.Version)
				}
			}
		})
	}
}

func TestReadGPIO(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expected    int
		expectError bool
	}{
		{
			name:     "Read 0",
			content:  "0\n",
			expected: 0,
		},
		{
			name:     "Read 1",
			content:  "1",
			expected: 1,
		},
		{
			name:        "Invalid content",
			content:     "invalid",
			expectError: true,
		},
		{
			name:        "Empty path",
			content:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Empty path" {
				_, err := readGPIO("")
				if err == nil {
					t.Error("Expected error for empty path")
				}
				return
			}

			tmpFile, err := os.CreateTemp("", "gpio")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.Write([]byte(tt.content)); err != nil {
				t.Fatal(err)
			}
			tmpFile.Close()

			result, err := readGPIO(tmpFile.Name())
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, got %d", tt.expected, result)
				}
			}
		})
	}
}

func TestWriteGPIO(t *testing.T) {
	tests := []struct {
		name        string
		duration    int
		expectError bool
	}{
		{
			name:     "No duration",
			duration: 0,
		},
		{
			name:     "With duration",
			duration: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "gpio")
			if err != nil {
				t.Fatal(err)
			}
			tmpFile.Close()
			defer os.Remove(tmpFile.Name())

			err = writeGPIO(tmpFile.Name(), tt.duration)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// After writeGPIO, the file should contain "0" (final state)
				content, err := os.ReadFile(tmpFile.Name())
				if err != nil {
					t.Fatal(err)
				}
				if string(content) != "0" {
					t.Errorf("Expected final GPIO state '0', got %s", content)
				}
			}
		})
	}

	t.Run("Empty path", func(t *testing.T) {
		err := writeGPIO("", 0)
		if err == nil {
			t.Error("Expected error for empty path")
		}
	})
}

func TestHandleServiceRoot(t *testing.T) {
	req, err := http.NewRequest("GET", "/redfish/v1", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleServiceRoot)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, status)
	}

	var root ServiceRoot
	if err := json.Unmarshal(rr.Body.Bytes(), &root); err != nil {
		t.Fatal(err)
	}

	if root.ID != "RootService" {
		t.Errorf("Expected ID 'RootService', got '%s'", root.ID)
	}
	if root.RedfishVersion != "1.8.0" {
		t.Errorf("Expected version '1.8.0', got '%s'", root.RedfishVersion)
	}
}

func TestHandleSystems(t *testing.T) {
	req, err := http.NewRequest("GET", "/redfish/v1/Systems", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleSystems)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, status)
	}

	var collection SystemCollection
	if err := json.Unmarshal(rr.Body.Bytes(), &collection); err != nil {
		t.Fatal(err)
	}

	if len(collection.Members) != 1 {
		t.Errorf("Expected 1 member, got %d", len(collection.Members))
	}
}

func TestHandleSystem(t *testing.T) {
	currentHardware = &HWAlpha

	tmpDir := t.TempDir()
	gpioFile := filepath.Join(tmpDir, "gpio_power_led")
	if err := os.WriteFile(gpioFile, []byte("0"), 0644); err != nil {
		t.Fatal(err)
	}
	
	oldPath := currentHardware.GPIOPowerLED
	currentHardware.GPIOPowerLED = gpioFile
	defer func() {
		currentHardware.GPIOPowerLED = oldPath
	}()

	req, err := http.NewRequest("GET", "/redfish/v1/Systems/System.1", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleSystem)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, status)
	}

	var system ComputerSystem
	if err := json.Unmarshal(rr.Body.Bytes(), &system); err != nil {
		t.Fatal(err)
	}

	if system.PowerState != "On" {
		t.Errorf("Expected PowerState 'On', got '%s'", system.PowerState)
	}

	// Test Boot properties
	if system.Boot.BootSourceOverrideEnabled == "" {
		t.Error("Boot.BootSourceOverrideEnabled should not be empty")
	}
	if len(system.Boot.BootSourceOverrideTargetAllowableValues) == 0 {
		t.Error("Boot.BootSourceOverrideTargetAllowableValues should not be empty")
	}
}

func TestHandleReset(t *testing.T) {
	currentHardware = &HWAlpha

	tmpDir := t.TempDir()
	gpioPower := filepath.Join(tmpDir, "gpio_power")
	gpioReset := filepath.Join(tmpDir, "gpio_reset")
	gpioPowerLED := filepath.Join(tmpDir, "gpio_power_led")
	
	if err := os.WriteFile(gpioPower, []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gpioReset, []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gpioPowerLED, []byte("0"), 0644); err != nil {
		t.Fatal(err)
	}

	oldPower := currentHardware.GPIOPower
	oldReset := currentHardware.GPIOReset
	oldPowerLED := currentHardware.GPIOPowerLED
	
	currentHardware.GPIOPower = gpioPower
	currentHardware.GPIOReset = gpioReset
	currentHardware.GPIOPowerLED = gpioPowerLED
	
	defer func() {
		currentHardware.GPIOPower = oldPower
		currentHardware.GPIOReset = oldReset
		currentHardware.GPIOPowerLED = oldPowerLED
	}()

	tests := []struct {
		name       string
		resetType  string
		expectCode int
	}{
		{
			name:       "ForceRestart",
			resetType:  "ForceRestart",
			expectCode: http.StatusNoContent,
		},
		{
			name:       "Invalid reset type",
			resetType:  "Invalid",
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := ResetRequest{ResetType: tt.resetType}
			jsonBody, _ := json.Marshal(body)
			
			req, err := http.NewRequest("POST", "/redfish/v1/Systems/System.1/Actions/ComputerSystem.Reset", bytes.NewBuffer(jsonBody))
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handleReset)
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectCode {
				t.Errorf("Expected status %d, got %d", tt.expectCode, status)
			}
		})
	}
}

func TestMethodNotAllowed(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		path    string
		handler http.HandlerFunc
	}{
		{
			name:    "POST to service root",
			method:  "POST",
			path:    "/redfish/v1",
			handler: handleServiceRoot,
		},
		{
			name:    "POST to systems",
			method:  "POST",
			path:    "/redfish/v1/Systems",
			handler: handleSystems,
		},
		{
			name:    "GET to reset action",
			method:  "GET",
			path:    "/redfish/v1/Systems/System.1/Actions/ComputerSystem.Reset",
			handler: handleReset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			tt.handler.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusMethodNotAllowed {
				t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, status)
			}
		})
	}
}

func TestInvalidJSON(t *testing.T) {
	req, err := http.NewRequest("POST", "/redfish/v1/Systems/System.1/Actions/ComputerSystem.Reset", 
		bytes.NewBufferString("invalid json"))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleReset)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, status)
	}
}

func TestHandleSystemPatch(t *testing.T) {
	// Reset boot config to default
	currentBootConfig = Boot{
		BootSourceOverrideEnabled: "Disabled",
		BootSourceOverrideMode:    "UEFI",
		BootSourceOverrideTarget:  "None",
		BootSourceOverrideTargetAllowableValues: []string{
			"None", "Pxe", "Cd", "Usb", "Hdd", "BiosSetup",
			"Utilities", "Diags", "UefiShell", "UefiTarget",
			"SDCard", "UefiHttp", "RemoteDrive", "UefiBootNext",
		},
	}

	tests := []struct {
		name       string
		body       string
		expectCode int
	}{
		{
			name: "Valid boot config update",
			body: `{
				"Boot": {
					"BootSourceOverrideEnabled": "Once",
					"BootSourceOverrideTarget": "Pxe"
				}
			}`,
			expectCode: http.StatusNoContent,
		},
		{
			name: "Invalid boot target",
			body: `{
				"Boot": {
					"BootSourceOverrideTarget": "InvalidTarget"
				}
			}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Invalid JSON",
			body:       "invalid json",
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("PATCH", "/redfish/v1/Systems/System.1", 
				bytes.NewBufferString(tt.body))
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handleSystem)
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectCode {
				t.Errorf("Expected status %d, got %d", tt.expectCode, status)
			}

			// Verify boot config was updated for valid request
			if tt.name == "Valid boot config update" && tt.expectCode == http.StatusNoContent {
				if currentBootConfig.BootSourceOverrideEnabled != "Once" {
					t.Errorf("Expected BootSourceOverrideEnabled 'Once', got '%s'", 
						currentBootConfig.BootSourceOverrideEnabled)
				}
				if currentBootConfig.BootSourceOverrideTarget != "Pxe" {
					t.Errorf("Expected BootSourceOverrideTarget 'Pxe', got '%s'", 
						currentBootConfig.BootSourceOverrideTarget)
				}
			}
		})
	}
}

func TestHandleManagers(t *testing.T) {
	req, err := http.NewRequest("GET", "/redfish/v1/Managers", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleManagers)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, status)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result["@odata.type"] != "#ManagerCollection.ManagerCollection" {
		t.Errorf("Expected ManagerCollection type, got %v", result["@odata.type"])
	}
}

func TestHandleChassis(t *testing.T) {
	req, err := http.NewRequest("GET", "/redfish/v1/Chassis", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleChassis)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, status)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result["@odata.type"] != "#ChassisCollection.ChassisCollection" {
		t.Errorf("Expected ChassisCollection type, got %v", result["@odata.type"])
	}
}