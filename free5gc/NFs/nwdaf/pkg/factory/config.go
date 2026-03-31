/*
 * NWDAF Configuration Factory
 */

package factory

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/asaskevich/govalidator"

	"github.com/free5gc/nwdaf/internal/logger"
)

const (
	NwdafDefaultTLSKeyLogPath           = "./log/nwdafsslkey.log"
	NwdafDefaultCertPemPath             = "./cert/nwdaf.pem"
	NwdafDefaultPrivateKeyPath          = "./cert/nwdaf.key"
	NwdafDefaultConfigPath              = "./config/nwdafcfg.yaml"
	NwdafSbiDefaultIPv4                 = "127.0.0.47"
	NwdafSbiDefaultPort                 = 8000
	NwdafSbiDefaultScheme               = "https"
	NwdafDefaultNrfUri                  = "https://127.0.0.10:8000"
	NwdafAnalyticsInfoResUriPrefix      = "/nnwdaf-analyticsinfo/v1"
	NwdafEventsSubscriptionResUriPrefix = "/nnwdaf-eventssubscription/v1"
	NwdafCallbackResUriPrefix           = "/nnwdaf-callback/v1"
	NwdafManagementResUriPrefix         = "/nnwdaf-management/v1"
)

type Config struct {
	Info          *Info          `yaml:"info" valid:"required"`
	Configuration *Configuration `yaml:"configuration" valid:"required"`
	Logger        *Logger        `yaml:"logger" valid:"required"`
	sync.RWMutex
}

func (c *Config) Validate() (bool, error) {
	if configuration := c.Configuration; configuration != nil {
		if result, err := configuration.validate(); err != nil {
			return result, err
		}
	}

	result, err := govalidator.ValidateStruct(c)
	return result, appendInvalid(err)
}

type Info struct {
	Version     string `yaml:"version,omitempty" valid:"required,in(1.0.0)"`
	Description string `yaml:"description,omitempty" valid:"type(string)"`
}

type Configuration struct {
	NrfUri            string                   `yaml:"nrfUri,omitempty" valid:"required,url"`
	NrfCertPem        string                   `yaml:"nrfCertPem,omitempty" valid:"optional"`
	Sbi               *Sbi                     `yaml:"sbi,omitempty" valid:"required"`
	ServiceNameList   []string                 `yaml:"serviceNameList,omitempty" valid:"required"`
	EventSubscription *EventSubscriptionConfig `yaml:"eventSubscription,omitempty" valid:"optional"`
}

func (c *Configuration) validate() (bool, error) {
	if c.Sbi != nil {
		if _, err := c.Sbi.validate(); err != nil {
			return false, err
		}
	}

	if c.ServiceNameList != nil {
		var errs govalidator.Errors
		for _, v := range c.ServiceNameList {
			if v != "nnwdaf-analyticsinfo" && v != "nnwdaf-eventssubscription" {
				err := fmt.Errorf("invalid ServiceNameList: %s,"+
					" value should be nnwdaf-analyticsinfo or nnwdaf-eventssubscription", v)
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			return false, error(errs)
		}
	}

	if _, err := govalidator.ValidateStruct(c); err != nil {
		return false, appendInvalid(err)
	}

	return true, nil
}

type Sbi struct {
	Scheme       string `yaml:"scheme" valid:"required,scheme"`
	RegisterIPv4 string `yaml:"registerIPv4,omitempty" valid:"required,host"`
	BindingIPv4  string `yaml:"bindingIPv4,omitempty" valid:"required,host"`
	Port         int    `yaml:"port,omitempty" valid:"required,port"`
	Tls          *Tls   `yaml:"tls,omitempty" valid:"optional"`
}

func (s *Sbi) validate() (bool, error) {
	govalidator.TagMap["scheme"] = govalidator.Validator(func(str string) bool {
		return str == "https" || str == "http"
	})

	if tls := s.Tls; tls != nil {
		if result, err := tls.validate(); err != nil {
			return result, err
		}
	}

	if _, err := govalidator.ValidateStruct(s); err != nil {
		return false, appendInvalid(err)
	}

	return true, nil
}

type Tls struct {
	Pem string `yaml:"pem,omitempty" valid:"type(string),minstringlength(1),required"`
	Key string `yaml:"key,omitempty" valid:"type(string),minstringlength(1),required"`
}

func (t *Tls) validate() (bool, error) {
	result, err := govalidator.ValidateStruct(t)
	return result, err
}

type EventSubscriptionConfig struct {
	AmfEventSubscription *NfEventSubscriptionConfig `yaml:"amfEventSubscription,omitempty" valid:"optional"`
	SmfEventSubscription *NfEventSubscriptionConfig `yaml:"smfEventSubscription,omitempty" valid:"optional"`
}

type NfEventSubscriptionConfig struct {
	NfUri  string   `yaml:"nfUri" valid:"required,url"`
	Events []string `yaml:"events" valid:"required"`
}

type Logger struct {
	Enable       bool   `yaml:"enable" valid:"type(bool)"`
	Level        string `yaml:"level" valid:"required,in(trace|debug|info|warn|error|fatal|panic)"`
	ReportCaller bool   `yaml:"reportCaller" valid:"type(bool)"`
}

func appendInvalid(err error) error {
	var errs govalidator.Errors

	if err == nil {
		return nil
	}

	es := err.(govalidator.Errors).Errors()
	for _, e := range es {
		errs = append(errs, fmt.Errorf("Invalid %w", e))
	}

	return error(errs)
}

func (c *Config) GetVersion() string {
	c.RLock()
	defer c.RUnlock()

	if c.Info != nil && c.Info.Version != "" {
		return c.Info.Version
	}
	return ""
}

func (c *Config) SetLogEnable(enable bool) {
	c.Lock()
	defer c.Unlock()

	if c.Logger == nil {
		logger.CfgLog.Warnf("Logger should not be nil")
		c.Logger = &Logger{
			Enable: enable,
			Level:  "info",
		}
	} else {
		c.Logger.Enable = enable
	}
}

func (c *Config) SetLogLevel(level string) {
	c.Lock()
	defer c.Unlock()

	if c.Logger == nil {
		logger.CfgLog.Warnf("Logger should not be nil")
		c.Logger = &Logger{
			Level: level,
		}
	} else {
		c.Logger.Level = level
	}
}

func (c *Config) SetLogReportCaller(reportCaller bool) {
	c.Lock()
	defer c.Unlock()

	if c.Logger == nil {
		logger.CfgLog.Warnf("Logger should not be nil")
		c.Logger = &Logger{
			Level:        "info",
			ReportCaller: reportCaller,
		}
	} else {
		c.Logger.ReportCaller = reportCaller
	}
}

func (c *Config) GetLogEnable() bool {
	c.RLock()
	defer c.RUnlock()

	if c.Logger == nil {
		logger.CfgLog.Warnf("Logger should not be nil")
		return false
	}
	return c.Logger.Enable
}

func (c *Config) GetLogLevel() string {
	c.RLock()
	defer c.RUnlock()

	if c.Logger == nil {
		logger.CfgLog.Warnf("Logger should not be nil")
		return "info"
	}
	return c.Logger.Level
}

func (c *Config) GetLogReportCaller() bool {
	c.RLock()
	defer c.RUnlock()

	if c.Logger == nil {
		logger.CfgLog.Warnf("Logger should not be nil")
		return false
	}
	return c.Logger.ReportCaller
}

func (c *Config) GetSbiScheme() string {
	if c.Configuration != nil && c.Configuration.Sbi != nil && c.Configuration.Sbi.Scheme != "" {
		return c.Configuration.Sbi.Scheme
	}
	return NwdafSbiDefaultScheme
}

func (c *Config) GetSbiPort() int {
	if c.Configuration != nil && c.Configuration.Sbi != nil && c.Configuration.Sbi.Port != 0 {
		return c.Configuration.Sbi.Port
	}
	return NwdafSbiDefaultPort
}

func (c *Config) GetSbiBindingIP() string {
	bindIP := "0.0.0.0"
	if c.Configuration == nil || c.Configuration.Sbi == nil {
		return bindIP
	}
	if c.Configuration.Sbi.BindingIPv4 != "" {
		if bindIP = os.Getenv(c.Configuration.Sbi.BindingIPv4); bindIP != "" {
			logger.CfgLog.Infof("Parsing ServerIPv4 [%s] from ENV Variable", bindIP)
		} else {
			bindIP = c.Configuration.Sbi.BindingIPv4
		}
	}
	return bindIP
}

func (c *Config) GetSbiBindingAddr() string {
	return c.GetSbiBindingIP() + ":" + strconv.Itoa(c.GetSbiPort())
}

func (c *Config) GetSbiRegisterIP() string {
	if c.Configuration != nil && c.Configuration.Sbi != nil && c.Configuration.Sbi.RegisterIPv4 != "" {
		return c.Configuration.Sbi.RegisterIPv4
	}
	return NwdafSbiDefaultIPv4
}

func (c *Config) GetSbiRegisterAddr() string {
	return c.GetSbiRegisterIP() + ":" + strconv.Itoa(c.GetSbiPort())
}

func (c *Config) GetSbiUri() string {
	return c.GetSbiScheme() + "://" + c.GetSbiRegisterAddr()
}

func (c *Config) GetNrfUri() string {
	if c.Configuration != nil && c.Configuration.NrfUri != "" {
		return c.Configuration.NrfUri
	}
	return NwdafDefaultNrfUri
}

func (c *Config) GetServiceNameList() []string {
	if c.Configuration != nil && len(c.Configuration.ServiceNameList) > 0 {
		return c.Configuration.ServiceNameList
	}
	return nil
}

func (c *Config) GetEventSubscription() *EventSubscriptionConfig {
	if c.Configuration != nil {
		return c.Configuration.EventSubscription
	}
	return nil
}

func (c *Config) GetCertPemPath() string {
	c.RLock()
	defer c.RUnlock()

	if c.Configuration != nil && c.Configuration.Sbi != nil && c.Configuration.Sbi.Tls != nil {
		return c.Configuration.Sbi.Tls.Pem
	}
	return NwdafDefaultCertPemPath
}

func (c *Config) GetCertKeyPath() string {
	c.RLock()
	defer c.RUnlock()

	if c.Configuration != nil && c.Configuration.Sbi != nil && c.Configuration.Sbi.Tls != nil {
		return c.Configuration.Sbi.Tls.Key
	}
	return NwdafDefaultPrivateKeyPath
}
