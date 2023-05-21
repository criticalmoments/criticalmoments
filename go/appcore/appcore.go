package appcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/CriticalMoments/CriticalMoments/go/cmcore"
	datamodel "github.com/CriticalMoments/CriticalMoments/go/cmcore/data_model"
)

func GoPing() string {
	return "AppcorePong->" + cmcore.CmCorePing()
}

type Appcore struct {
	// Library binding/delegate
	libBindings LibBindings

	// Primary configuration
	configUrlString string
	config          *datamodel.PrimaryConfig

	// Cache
	cache *cache

	// Properties
	propertyRegistry *propertyRegistry
}

var sharedAppcore Appcore = newAppcore()

func SharedAppcore() *Appcore {
	return &sharedAppcore
}
func newAppcore() Appcore {
	return Appcore{
		propertyRegistry: newPropertyRegistry(),
	}
}

// Hopefully no one wants http (no TLS) in 2023... but given the importance of the config file we can't open this up to injection attacks
const filePrefix = "file://"
const httpsPrefix = "https://"

func (ac *Appcore) SetConfigUrl(configUrl string) error {
	if !strings.HasPrefix(configUrl, filePrefix) && !strings.HasPrefix(configUrl, httpsPrefix) {
		return errors.New("Config URL must start with https:// or file://")
	}
	ac.configUrlString = configUrl

	return nil
}

func (ac *Appcore) SetCacheDirPath(cacheDirPath string) error {
	cache, err := newCacheWithBaseDir(cacheDirPath)
	if err != nil {
		return err
	}

	ac.cache = cache
	return nil
}

func (ac *Appcore) RegisterLibraryBindings(lb LibBindings) {
	ac.libBindings = lb
}

// TODO: guard against double start call
func (ac *Appcore) Start() error {
	if ac.configUrlString == "" {
		return errors.New("A config URL must be provided before starting critical moments")
	}
	if ac.libBindings == nil {
		return errors.New("The SDK must register LibBindings before calling start")
	}
	if ac.cache == nil {
		return errors.New("The SDK must register a cache directory before calling start")
	}
	// TODO: not fatal error? Loud in dev mode but not fatal.
	if ac.propertyRegistry.validatePropertiesReturningUserReadable() != "" {
		return errors.New(ac.propertyRegistry.validatePropertiesReturningUserReadable())
	}

	var configFilePath string
	var err error

	if strings.HasPrefix(ac.configUrlString, filePrefix) {
		// Strip file:// prefix
		configFilePath = ac.configUrlString[len(filePrefix):]
	} else if strings.HasPrefix(ac.configUrlString, httpsPrefix) {
		configFilePath, err = ac.cache.verifyOrFetchRemoteConfigFile(ac.configUrlString, "primary")
		if err != nil {
			return err
		}
	}
	if configFilePath == "" {
		return errors.New("CriticalMoments: Invalid config url")
	}

	configFileData, err := os.ReadFile(configFilePath)
	if err != nil {
		return err
	}
	var pc datamodel.PrimaryConfig
	err = json.Unmarshal(configFileData, &pc)
	if err != nil {
		return err
	}
	ac.config = &pc
	err = ac.postConfigSetup()
	if err != nil {
		return err
	}

	return nil
}

func (ac *Appcore) postConfigSetup() error {
	if ac.config.DefaultTheme != nil {
		err := ac.libBindings.SetDefaultTheme(ac.config.DefaultTheme)
		if err != nil {
			fmt.Println("CriticalMoments: there was an issue setting up the default theme from config")
			return err
		}
	}

	return nil
}

// TODO: method considered WIP, not tested, expect a full re-write for conditions so saving for later
// TODO: events should be queued during setup, and run after postConfigSetup
func (ac *Appcore) SendEvent(e string) {
	actions := ac.config.ActionsForEvent(e)
	for _, action := range actions {
		err := dispatchActionToLib(&action, ac.libBindings)
		if err != nil {
			fmt.Printf("CriticalMoments: there was an issue performing action for event \"%v\". Error: %v\n", e, err)
		}
	}
}

func (ac *Appcore) PerformNamedAction(actionName string) error {
	action := ac.config.ActionWithName(actionName)
	if action == nil {
		return errors.New(fmt.Sprintf("No action found named %v", actionName))
	}
	return action.PerformAction(ac.libBindings)
}

func (ac *Appcore) ThemeForName(themeName string) *datamodel.Theme {
	return ac.config.ThemeWithName(themeName)
}

// Repeitive, but gomobile doesn't allow for `interface{}`
func (ac *Appcore) RegisterStaticStringProperty(key string, value string) {
	ac.propertyRegistry.registerStaticProperty(key, value)
}
func (ac *Appcore) RegisterStaticIntProperty(key string, value int) {
	ac.propertyRegistry.registerStaticProperty(key, value)
}
func (ac *Appcore) RegisterStaticFloatProperty(key string, value float64) {
	ac.propertyRegistry.registerStaticProperty(key, value)
}
func (ac *Appcore) RegisterStaticVersionNumberProperty(prefix string, versionString string) error {
	return ac.propertyRegistry.registerStaticVersionNumberProperty(prefix, versionString)
}
