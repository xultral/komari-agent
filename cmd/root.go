package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"syscall"

	"github.com/xultral/komari-agent/dnsresolver"
	pkg_flags "github.com/xultral/komari-agent/cmd/flags"
	"github.com/xultral/komari-agent/monitoring/netstatic"
	monitoring "github.com/xultral/komari-agent/monitoring/unit"
	"github.com/xultral/komari-agent/server"
	"github.com/xultral/komari-agent/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var flags = pkg_flags.GlobalConfig

var RootCmd = &cobra.Command{
	Use:   "komari-agent",
	Short: "komari agent",
	Long:  `komari agent (monitoring-only)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadConfig(cmd); err != nil {
			return err
		}
		if flags.PreferIPVersion != "" && flags.PreferIPVersion != "4" && flags.PreferIPVersion != "6" {
			return fmt.Errorf("invalid --prefer-ip-version value %q: expected 4 or 6", flags.PreferIPVersion)
		}

		stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		go func() {
			<-stopCtx.Done()
			log.Printf("shutting down gracefully...")
			netstatic.Stop()
			os.Exit(0)
		}()

		if flags.MonthRotate != 0 {
			err := netstatic.StartOrContinue()
			if err != nil {
				log.Println("Failed to start netstatic monitoring:", err)
			}
			nics, err := monitoring.InterfaceList()
			if err != nil {
				log.Println("Failed to get interface list for netstatic:", err)
			}
			err = netstatic.SetNewConfig(netstatic.NetStaticConfig{
				Nics: nics,
			})
			if err != nil {
				log.Println("Failed to set netstatic config:", err)
			}
		}

		log.Println("Komari Agent", version.CurrentVersion)
		log.Println("Mode: monitoring-only; remote tasks, web terminal, and self-update are disabled")

		if flags.CustomDNS != "" {
			dnsresolver.SetCustomDNSServer(flags.CustomDNS)
			log.Printf("Using custom DNS server: %s", flags.CustomDNS)
		} else {
			log.Printf("Using system default DNS resolver")
		}

		if flags.AutoDiscoveryKey != "" {
			err := handleAutoDiscovery()
			if err != nil {
				return fmt.Errorf("auto-discovery failed: %w", err)
			}
		}
		diskList, err := monitoring.DiskList()
		if err != nil {
			log.Println("Failed to get disk list:", err)
		}
		log.Println("Monitoring Mountpoints:", diskList)
		interfaceList, err := monitoring.InterfaceList()
		if err != nil {
			log.Println("Failed to get interface list:", err)
		}
		log.Println("Monitoring Interfaces:", interfaceList)

		if flags.IgnoreUnsafeCert {
			http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}

		go server.DoUploadBasicInfoWorks()
		for {
			server.UpdateBasicInfo()
			server.EstablishWebSocketConnection()
		}
	},
}

func Execute() {
	for i, arg := range os.Args {
		if arg == "-autoUpdate" || arg == "--autoUpdate" {
			log.Println("WARNING: Automatic updates are permanently disabled in monitoring-only mode.")
			os.Args = append(os.Args[:i], os.Args[i+1:]...)
			break
		}
		if arg == "-memory-mode-available" || arg == "--memory-mode-available" {
			log.Println("WARNING: The --memory-mode-available flag is deprecated in version 1.0.70 and later. Use --memory-include-cache to report memory usage including cache/buffer.")
			os.Args = append(os.Args[:i], os.Args[i+1:]...)
		}
	}

	if err := RootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func init() {
	registerPersistentFlags(RootCmd)
}

func registerPersistentFlags(cmd *cobra.Command) {
	defaults := defaultConfig()

	cmd.PersistentFlags().StringVarP(&flags.Token, "token", "t", "", "API token")
	cmd.PersistentFlags().StringVarP(&flags.Endpoint, "endpoint", "e", "", "API endpoint")
	cmd.PersistentFlags().StringVar(&flags.AutoDiscoveryKey, "auto-discovery", "", "Auto discovery key for the agent")
	cmd.PersistentFlags().BoolVar(&flags.DisableAutoUpdate, "disable-auto-update", defaults.DisableAutoUpdate, "Deprecated compatibility flag. Automatic updates are permanently disabled.")
	cmd.PersistentFlags().BoolVar(&flags.DisableWebSsh, "disable-web-ssh", defaults.DisableWebSsh, "Deprecated compatibility flag. Remote control is permanently disabled.")
	cmd.PersistentFlags().Float64VarP(&flags.Interval, "interval", "i", defaults.Interval, "Interval in seconds")
	cmd.PersistentFlags().BoolVarP(&flags.IgnoreUnsafeCert, "ignore-unsafe-cert", "u", false, "Ignore unsafe certificate errors")
	cmd.PersistentFlags().IntVarP(&flags.MaxRetries, "max-retries", "r", defaults.MaxRetries, "Maximum number of retries")
	cmd.PersistentFlags().IntVarP(&flags.ReconnectInterval, "reconnect-interval", "c", defaults.ReconnectInterval, "Reconnect interval in seconds")
	cmd.PersistentFlags().IntVar(&flags.InfoReportInterval, "info-report-interval", defaults.InfoReportInterval, "Interval in minutes for reporting basic info")
	cmd.PersistentFlags().StringVar(&flags.IncludeNics, "include-nics", "", "Comma-separated list of network interfaces to include")
	cmd.PersistentFlags().StringVar(&flags.ExcludeNics, "exclude-nics", "", "Comma-separated list of network interfaces to exclude")
	cmd.PersistentFlags().StringVar(&flags.IncludeMountpoints, "include-mountpoint", "", "Semicolon-separated list of mount points to include for disk statistics")
	cmd.PersistentFlags().IntVar(&flags.MonthRotate, "month-rotate", 0, "Month reset for network statistics (0 to disable)")
	cmd.PersistentFlags().StringVar(&flags.CFAccessClientID, "cf-access-client-id", "", "Cloudflare Access Client ID")
	cmd.PersistentFlags().StringVar(&flags.CFAccessClientSecret, "cf-access-client-secret", "", "Cloudflare Access Client Secret")
	cmd.PersistentFlags().BoolVar(&flags.MemoryIncludeCache, "memory-include-cache", false, "Include cache/buffer in memory usage")
	cmd.PersistentFlags().BoolVar(&flags.MemoryReportRawUsed, "memory-exclude-bcf", false, "Use \"raminfo.Used = v.Total - v.Free - v.Buffers - v.Cached\" calculation for memory usage")
	cmd.PersistentFlags().StringVar(&flags.CustomDNS, "custom-dns", "", "Custom DNS server to use (e.g. 8.8.8.8, 114.114.114.114). By default, the program uses the system DNS resolver.")
	cmd.PersistentFlags().BoolVar(&flags.EnableGPU, "gpu", false, "Enable detailed GPU monitoring (usage, memory, multi-GPU support)")
	cmd.PersistentFlags().BoolVar(&flags.ShowWarning, "show-warning", false, "Deprecated compatibility flag. Does nothing in monitoring-only mode.")
	cmd.PersistentFlags().StringVar(&flags.CustomIpv4, "custom-ipv4", "", "Custom IPv4 address to use")
	cmd.PersistentFlags().StringVar(&flags.CustomIpv6, "custom-ipv6", "", "Custom IPv6 address to use")
	cmd.PersistentFlags().BoolVar(&flags.GetIpAddrFromNic, "get-ip-addr-from-nic", false, "Get IP address from network interface")
	cmd.PersistentFlags().StringVar(&flags.ConfigFile, "config", "", "Path to the configuration file")
	cmd.PersistentFlags().IntVar(&flags.ProtocolVersion, "protocol-version", defaults.ProtocolVersion, "Report protocol version (1 or 2)")
	cmd.PersistentFlags().BoolVar(&flags.DisableCompression, "disable-compression", false, "Disable v2 gzip/permessage-deflate compression")
	cmd.PersistentFlags().StringVar(&flags.PreferIPVersion, "prefer-ip-version", "", "Prefer IP version for dashboard connections: 4 or 6")
	cmd.PersistentFlags().ParseErrorsWhitelist.UnknownFlags = true
}

func defaultConfig() pkg_flags.Config {
	return pkg_flags.Config{
		DisableAutoUpdate: true,
		DisableWebSsh:     true,
		Interval:          1.0,
		MaxRetries:        3,
		ReconnectInterval: 5,
		InfoReportInterval: 5,
		ProtocolVersion:   2,
	}
}

func loadConfig(cmd *cobra.Command) error {
	resolved := defaultConfig()
	configPath := resolved.ConfigFile

	if envConfigPath := os.Getenv("AGENT_CONFIG_FILE"); envConfigPath != "" {
		configPath = envConfigPath
	}
	if flag := cmd.Flags().Lookup("config"); flag != nil && flag.Changed {
		configPath = flag.Value.String()
	}
	resolved.ConfigFile = configPath

	if configPath != "" {
		bytes, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		if err := json.Unmarshal(bytes, &resolved); err != nil {
			return fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	loadFromEnv(&resolved)
	applyFlagOverrides(cmd, &resolved)
	enforceMonitoringOnlyDefaults(&resolved)
	*flags = resolved

	return nil
}

func loadFromEnv(target *pkg_flags.Config) {
	val := reflect.ValueOf(target).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			continue
		}

		envValue := os.Getenv(envTag)
		if envValue == "" {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			field.SetString(envValue)
		case reflect.Bool:
			if boolVal, err := strconv.ParseBool(envValue); err == nil {
				field.SetBool(boolVal)
			}
		case reflect.Int:
			if intVal, err := strconv.Atoi(envValue); err == nil {
				field.SetInt(int64(intVal))
			}
		case reflect.Float64:
			if floatVal, err := strconv.ParseFloat(envValue, 64); err == nil {
				field.SetFloat(floatVal)
			}
		}
	}
}

func applyFlagOverrides(cmd *cobra.Command, target *pkg_flags.Config) {
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		switch flag.Name {
		case "token":
			target.Token = flag.Value.String()
		case "endpoint":
			target.Endpoint = flag.Value.String()
		case "auto-discovery":
			target.AutoDiscoveryKey = flag.Value.String()
		case "disable-auto-update":
			if value, err := strconv.ParseBool(flag.Value.String()); err == nil {
				target.DisableAutoUpdate = value
			}
		case "disable-web-ssh":
			if value, err := strconv.ParseBool(flag.Value.String()); err == nil {
				target.DisableWebSsh = value
			}
		case "interval":
			if value, err := strconv.ParseFloat(flag.Value.String(), 64); err == nil {
				target.Interval = value
			}
		case "ignore-unsafe-cert":
			if value, err := strconv.ParseBool(flag.Value.String()); err == nil {
				target.IgnoreUnsafeCert = value
			}
		case "max-retries":
			if value, err := strconv.Atoi(flag.Value.String()); err == nil {
				target.MaxRetries = value
			}
		case "reconnect-interval":
			if value, err := strconv.Atoi(flag.Value.String()); err == nil {
				target.ReconnectInterval = value
			}
		case "info-report-interval":
			if value, err := strconv.Atoi(flag.Value.String()); err == nil {
				target.InfoReportInterval = value
			}
		case "include-nics":
			target.IncludeNics = flag.Value.String()
		case "exclude-nics":
			target.ExcludeNics = flag.Value.String()
		case "include-mountpoint":
			target.IncludeMountpoints = flag.Value.String()
		case "month-rotate":
			if value, err := strconv.Atoi(flag.Value.String()); err == nil {
				target.MonthRotate = value
			}
		case "cf-access-client-id":
			target.CFAccessClientID = flag.Value.String()
		case "cf-access-client-secret":
			target.CFAccessClientSecret = flag.Value.String()
		case "memory-include-cache":
			if value, err := strconv.ParseBool(flag.Value.String()); err == nil {
				target.MemoryIncludeCache = value
			}
		case "memory-exclude-bcf":
			if value, err := strconv.ParseBool(flag.Value.String()); err == nil {
				target.MemoryReportRawUsed = value
			}
		case "custom-dns":
			target.CustomDNS = flag.Value.String()
		case "gpu":
			if value, err := strconv.ParseBool(flag.Value.String()); err == nil {
				target.EnableGPU = value
			}
		case "show-warning":
			if value, err := strconv.ParseBool(flag.Value.String()); err == nil {
				target.ShowWarning = value
			}
		case "custom-ipv4":
			target.CustomIpv4 = flag.Value.String()
		case "custom-ipv6":
			target.CustomIpv6 = flag.Value.String()
		case "get-ip-addr-from-nic":
			if value, err := strconv.ParseBool(flag.Value.String()); err == nil {
				target.GetIpAddrFromNic = value
			}
		case "config":
			target.ConfigFile = flag.Value.String()
		case "protocol-version":
			if value, err := strconv.Atoi(flag.Value.String()); err == nil {
				target.ProtocolVersion = value
			}
		case "disable-compression":
			if value, err := strconv.ParseBool(flag.Value.String()); err == nil {
				target.DisableCompression = value
			}
		case "prefer-ip-version":
			target.PreferIPVersion = flag.Value.String()
		}
	})
}

func enforceMonitoringOnlyDefaults(target *pkg_flags.Config) {
	target.DisableWebSsh = true
	target.DisableAutoUpdate = true
	target.ShowWarning = false
}
