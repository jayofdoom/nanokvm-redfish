package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type HWVersion string

const (
	HWVersionAlpha HWVersion = "alpha"
	HWVersionBeta  HWVersion = "beta"
	HWVersionPcie  HWVersion = "pcie"
)

type Hardware struct {
	Version      HWVersion
	GPIOReset    string
	GPIOPower    string
	GPIOPowerLED string
	GPIOHDDLed   string
}

var HWAlpha = Hardware{
	Version:      HWVersionAlpha,
	GPIOReset:    "/sys/class/gpio/gpio507/value",
	GPIOPower:    "/sys/class/gpio/gpio503/value",
	GPIOPowerLED: "/sys/class/gpio/gpio504/value",
	GPIOHDDLed:   "/sys/class/gpio/gpio505/value",
}

var HWBeta = Hardware{
	Version:      HWVersionBeta,
	GPIOReset:    "/sys/class/gpio/gpio505/value",
	GPIOPower:    "/sys/class/gpio/gpio503/value",
	GPIOPowerLED: "/sys/class/gpio/gpio504/value",
	GPIOHDDLed:   "",
}

var HWPcie = Hardware{
	Version:      HWVersionPcie,
	GPIOReset:    "/sys/class/gpio/gpio505/value",
	GPIOPower:    "/sys/class/gpio/gpio503/value",
	GPIOPowerLED: "/sys/class/gpio/gpio504/value",
	GPIOHDDLed:   "",
}

var currentHardware *Hardware
var hwVersionFile = "/etc/kvm/hw"

func detectHardware() (*Hardware, error) {
	return detectHardwareFromFile(hwVersionFile)
}

func detectHardwareFromFile(path string) (*Hardware, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read hardware version: %w", err)
	}

	version := strings.TrimSpace(string(content))
	switch version {
	case "alpha":
		return &HWAlpha, nil
	case "beta":
		return &HWBeta, nil
	case "pcie":
		return &HWPcie, nil
	default:
		return nil, fmt.Errorf("unknown hardware version: %s", version)
	}
}

func readGPIO(path string) (int, error) {
	if path == "" {
		return 0, fmt.Errorf("GPIO path not available for this hardware")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failed to read GPIO: %w", err)
	}

	value, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		return 0, fmt.Errorf("failed to parse GPIO value: %w", err)
	}

	return value, nil
}

func writeGPIO(path string, value int) error {
	if path == "" {
		return fmt.Errorf("GPIO path not available for this hardware")
	}

	data := []byte(fmt.Sprintf("%d", value))
	return os.WriteFile(path, data, 0644)
}

func getPowerState() (string, error) {
	powerLED, err := readGPIO(currentHardware.GPIOPowerLED)
	if err != nil {
		return "", err
	}

	if powerLED == 1 {
		return "On", nil
	}
	return "Off", nil
}

func performReset() error {
	if err := writeGPIO(currentHardware.GPIOReset, 0); err != nil {
		return err
	}
	return writeGPIO(currentHardware.GPIOReset, 1)
}

func pressPowerButton() error {
	if err := writeGPIO(currentHardware.GPIOPower, 0); err != nil {
		return err
	}
	return writeGPIO(currentHardware.GPIOPower, 1)
}

type ServiceRoot struct {
	ODataType    string                 `json:"@odata.type"`
	ODataID      string                 `json:"@odata.id"`
	ID           string                 `json:"Id"`
	Name         string                 `json:"Name"`
	RedfishVersion string              `json:"RedfishVersion"`
	Systems      map[string]string      `json:"Systems"`
}

type SystemCollection struct {
	ODataType string                 `json:"@odata.type"`
	ODataID   string                 `json:"@odata.id"`
	Name      string                 `json:"Name"`
	Members   []map[string]string    `json:"Members"`
}

type ComputerSystem struct {
	ODataType    string                 `json:"@odata.type"`
	ODataID      string                 `json:"@odata.id"`
	ID           string                 `json:"Id"`
	Name         string                 `json:"Name"`
	PowerState   string                 `json:"PowerState"`
	Actions      map[string]interface{} `json:"Actions"`
}

type ResetAction struct {
	Target               string   `json:"target"`
	ResetTypeRedfishAllowableValues []string `json:"ResetType@Redfish.AllowableValues"`
}

type ResetRequest struct {
	ResetType string `json:"ResetType"`
}

func handleServiceRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	root := ServiceRoot{
		ODataType:      "#ServiceRoot.v1_5_0.ServiceRoot",
		ODataID:        "/redfish/v1",
		ID:             "RootService",
		Name:           "NanoKVM Redfish Service",
		RedfishVersion: "1.8.0",
		Systems: map[string]string{
			"@odata.id": "/redfish/v1/Systems",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(root)
}

func handleSystems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	collection := SystemCollection{
		ODataType: "#ComputerSystemCollection.ComputerSystemCollection",
		ODataID:   "/redfish/v1/Systems",
		Name:      "Computer System Collection",
		Members: []map[string]string{
			{"@odata.id": "/redfish/v1/Systems/System.1"},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(collection)
}

func handleSystem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	powerState, err := getPowerState()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get power state: %v", err), http.StatusInternalServerError)
		return
	}

	system := ComputerSystem{
		ODataType:  "#ComputerSystem.v1_13_0.ComputerSystem",
		ODataID:    "/redfish/v1/Systems/System.1",
		ID:         "System.1",
		Name:       "NanoKVM System",
		PowerState: powerState,
		Actions: map[string]interface{}{
			"#ComputerSystem.Reset": ResetAction{
				Target: "/redfish/v1/Systems/System.1/Actions/ComputerSystem.Reset",
				ResetTypeRedfishAllowableValues: []string{"On", "ForceOff", "GracefulShutdown", "ForceRestart"},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(system)
}

func handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ResetRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	switch req.ResetType {
	case "On":
		powerState, _ := getPowerState()
		if powerState == "Off" {
			if err := pressPowerButton(); err != nil {
				http.Error(w, fmt.Sprintf("Failed to power on: %v", err), http.StatusInternalServerError)
				return
			}
		}
	case "ForceOff":
		powerState, _ := getPowerState()
		if powerState == "On" {
			if err := pressPowerButton(); err != nil {
				http.Error(w, fmt.Sprintf("Failed to power off: %v", err), http.StatusInternalServerError)
				return
			}
		}
	case "GracefulShutdown":
		powerState, _ := getPowerState()
		if powerState == "On" {
			if err := pressPowerButton(); err != nil {
				http.Error(w, fmt.Sprintf("Failed to shutdown: %v", err), http.StatusInternalServerError)
				return
			}
		}
	case "ForceRestart":
		if err := performReset(); err != nil {
			http.Error(w, fmt.Sprintf("Failed to reset: %v", err), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, fmt.Sprintf("Invalid ResetType: %s", req.ResetType), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func main() {
	hw, err := detectHardware()
	if err != nil {
		log.Fatalf("Failed to detect hardware: %v", err)
	}
	currentHardware = hw
	log.Printf("Detected hardware version: %s", hw.Version)

	http.HandleFunc("/redfish/v1", handleServiceRoot)
	http.HandleFunc("/redfish/v1/", handleServiceRoot)
	http.HandleFunc("/redfish/v1/Systems", handleSystems)
	http.HandleFunc("/redfish/v1/Systems/", handleSystems)
	http.HandleFunc("/redfish/v1/Systems/System.1", handleSystem)
	http.HandleFunc("/redfish/v1/Systems/System.1/", handleSystem)
	http.HandleFunc("/redfish/v1/Systems/System.1/Actions/ComputerSystem.Reset", handleReset)

	port := ":8080"
	log.Printf("Starting Redfish API server on %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}