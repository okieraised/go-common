package go_common

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

var (
	once sync.Once
)

func init() {
	once.Do(func() {
		err := Load()
		if err != nil {
			panic(fmt.Sprintf("Failed to setup service configuration: %v", err))
		}
	})
}

func Load() error {
	err := godotenv.Load(".env")
	if err != nil {
		return err
	}

	profile := detectProfile()
	err = godotenv.Overload("." + profile + ".env")
	if err != nil {
		return err
	}

	v := viper.New()

	v.SetConfigFile("conf/config.toml")
	err = v.ReadInConfig()
	if err != nil {
		return err
	}

	v.SetConfigFile("conf/" + profile + ".config.toml")
	err = v.MergeInConfig()
	if err != nil {
		return err
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "__"))
	v.AutomaticEnv()
	return nil
}

// Detects the active configuration profile.
// Precedence: APP_ENV > GO_ENV > ENV > (debug=dev, release=prod)
func detectProfile() string {
	from := func(k string) (string, bool) {
		if v, ok := os.LookupEnv(k); ok {
			return strings.ToLower(v), true
		}
		return "", false
	}

	if v, ok := from("APP_ENV"); ok {
		return v
	}
	if v, ok := from("GO_ENV"); ok {
		return v
	}
	if v, ok := from("ENV"); ok {
		return v
	}
	return "dev"
}
