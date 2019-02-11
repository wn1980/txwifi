package iotwifi

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/bhoriuchi/go-bunyan/bunyan"
)

// WpaCfg for configuring wpa
type WpaCfg struct {
	Log    bunyan.Logger
	WpaCmd []string
	WpaCfg *SetupCfg
}

// WpaNetwork defines a wifi network to connect to.
type WpaNetwork struct {
	Bssid       string `json:"bssid"`
	Frequency   string `json:"frequency"`
	SignalLevel string `json:"signal_level"`
	Flags       string `json:"flags"`
	Ssid        string `json:"ssid"`
}

// WpaCredentials defines wifi network credentials.
type WpaCredentials struct {
	Ssid string `json:"ssid"`
	Psk  string `json:"psk"`
}

// WpaConnection defines a WPA connection.
type WpaConnection struct {
	Ssid    string `json:"ssid"`
	State   string `json:"state"`
	Ip      string `json:"ip"`
	Message string `json:"message"`
}

// NewWpaCfg produces WpaCfg configuration types.
func NewWpaCfg(log bunyan.Logger, cfgLocation string) *WpaCfg {

	setupCfg, err := loadCfg(cfgLocation)
	if err != nil {
		log.Error("Could not load config: %s", err.Error())
		panic(err)
	}

	return &WpaCfg{
		Log:    log,
		WpaCfg: setupCfg,
	}
}

// ConfiguredNetworks returns a list of configured wifi networks.
func (wpa *WpaCfg) ConfiguredNetworks() string {
	netOut, err := exec.Command("wpa_cli", "-i", "wlan0", "scan").Output()
	if err != nil {
		wpa.Log.Fatal(err)
	}

	return string(netOut)
}

func interfaceState(iface string) string {
	// regex for state
	rState := regexp.MustCompile("(?m)wpa_state=(.*)\n")
	stateOut, err := exec.Command("wpa_cli", "-i", iface, "status").Output()
	if err != nil {
		return "none"
	}
	ms := rState.FindSubmatch(stateOut)
	if len(ms) > 0 {
		state := string(ms[1])
		// see https://developer.android.com/reference/android/net/wifi/SupplicantState.html
		return state
	}
	return "none"
}

// ConnectNetwork connects to a wifi network
func (wpa *WpaCfg) ConnectNetwork(creds WpaCredentials) (WpaConnection, error) {
	connection := WpaConnection{}

	// 1. Add a network
	addNetOut, err := exec.Command("wpa_cli", "-i", "wlan0", "add_network").Output()
	if err != nil {
		wpa.Log.Fatal(err)
		return connection, err
	}
	net := strings.TrimSpace(string(addNetOut))
	wpa.Log.Info("WPA add network got: %s", net)

	// 2. Set the ssid for the new network
	addSsidOut, err := exec.Command("wpa_cli", "-i", "wlan0", "set_network", net, "ssid", "\""+creds.Ssid+"\"").Output()
	if err != nil {
		wpa.Log.Fatal(err)
		return connection, err
	}
	ssidStatus := strings.TrimSpace(string(addSsidOut))
	wpa.Log.Info("WPA add ssid got: %s", ssidStatus)

	// 3. Set the psk for the new network
	addPskOut, err := exec.Command("wpa_cli", "-i", "wlan0", "set_network", net, "psk", "\""+creds.Psk+"\"").Output()
	if err != nil {
		wpa.Log.Fatal(err.Error())
		return connection, err
	}
	pskStatus := strings.TrimSpace(string(addPskOut))
	wpa.Log.Info("WPA psk got: %s", pskStatus)

	// 4. Enable the new network
	enableOut, err := exec.Command("wpa_cli", "-i", "wlan0", "enable_network", net).Output()
	if err != nil {
		wpa.Log.Fatal(err.Error())
		return connection, err
	}
	enableStatus := strings.TrimSpace(string(enableOut))
	wpa.Log.Info("WPA enable got: %s", enableStatus)

	// regex for state
	rState := regexp.MustCompile("(?m)wpa_state=(.*)\n")

	// loop for status every second
	for i := 0; i < 5; i++ {
		wpa.Log.Info("WPA Checking wifi state")

		stateOut, err := exec.Command("wpa_cli", "-i", "wlan0", "status").Output()
		if err != nil {
			wpa.Log.Fatal("Got error checking state: %s", err.Error())
			return connection, err
		}
		ms := rState.FindSubmatch(stateOut)

		if len(ms) > 0 {
			state := string(ms[1])
			wpa.Log.Info("WPA Enable state: %s", state)
			// see https://developer.android.com/reference/android/net/wifi/SupplicantState.html
			if state == "COMPLETED" {
				// save the config
				saveOut, err := exec.Command("wpa_cli", "-i", "wlan0", "save_config").Output()
				if err != nil {
					wpa.Log.Fatal(err.Error())
					return connection, err
				}
				saveStatus := strings.TrimSpace(string(saveOut))
				wpa.Log.Info("WPA save got: %s", saveStatus)

				connection.Ssid = creds.Ssid
				connection.State = state

				return connection, nil
			}
		}

		time.Sleep(3 * time.Second)
	}

	connection.State = "FAIL"
	connection.Message = "Unable to connection to " + creds.Ssid
	return connection, nil
}

// Status returns the WPA wireless status.
func (wpa *WpaCfg) Status() (map[string]string, error) {
	cfgMap := make(map[string]string, 0)

	stateOut, err := exec.Command("wpa_cli", "-i", "wlan0", "status").Output()
	if err != nil {
		wpa.Log.Fatal("Got error checking state: %s", err.Error())
		return cfgMap, err
	}

	cfgMap = cfgMapper(stateOut)

	return cfgMap, nil
}

// cfgMapper takes a byte array and splits by \n and then by = and puts it all in a map.
func cfgMapper(data []byte) map[string]string {
	cfgMap := make(map[string]string, 0)

	lines := bytes.Split(data, []byte("\n"))

	for _, line := range lines {
		kv := bytes.Split(line, []byte("="))
		if len(kv) > 1 {
			cfgMap[string(kv[0])] = string(kv[1])
		}
	}

	return cfgMap
}

// ScanNetworks returns a map of WpaNetwork data structures.
func (wpa *WpaCfg) ScanNetworks() (map[string]WpaNetwork, error) {
	wpaNetworks := make(map[string]WpaNetwork, 0)
	bssid := ""
	ssid := ""
	freq := ""
	flags := ""
	signalLevel := ""
	networkListOut, err := exec.Command("iwlist", "uap0", "scan").Output()
	if err != nil {
		wpa.Log.Fatal(err)
		return wpaNetworks, err
	}

	networkListOutArr := strings.Split(string(networkListOut), "\n")
	fieldsFound := 0
	for _, netListOutLine := range networkListOutArr[1:] {
		//           Cell 01 - Address: xx:yy:zz:aa:bb:cc
		if strings.Contains(netListOutLine, "Address") {
			bssid = strings.Fields(netListOutLine)[4]
			fieldsFound = 1
		}
		//                    Frequency:5.785 GHz
		if strings.Contains(netListOutLine, "Frequency") {
			freq = strings.Split(strings.Split(netListOutLine, ":")[1], " ")[0]
			fieldsFound++
		}
		//                    Quality=69/70  Signal level=-41 dBm
		if strings.Contains(netListOutLine, "Signal level") {
			signalLevel = strings.Split(netListOutLine, "=")[2]
			fieldsFound++
		}
		//                    ESSID:"networkname"
		//This will break in the ?unlikely? event that the ESSID
		//  has a " in it.
		if strings.Contains(netListOutLine, "ESSID") {
			ssid = strings.Split(netListOutLine, "\"")[1]
			fieldsFound++
		}
		//                    IE: IEEE 802.11i/WPA2 Version 1
		if strings.Contains(netListOutLine, "IEEE") {
			flags = strings.Split(netListOutLine, "IEEE ")[1]
			fieldsFound++
		}

		if fieldsFound == 5 {
			wpaNetworks[ssid] = WpaNetwork{
				Bssid:       bssid,
				Frequency:   freq,
				SignalLevel: signalLevel,
				Flags:       flags,
				Ssid:        ssid,
			}
			bssid = ""
			ssid = ""
			freq = ""
			flags = ""
			signalLevel = ""
		}
	}
	return wpaNetworks, nil
}
