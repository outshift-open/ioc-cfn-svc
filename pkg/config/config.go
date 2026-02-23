package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/namsral/flag"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

var log = logger.SubPkg("config")

var (
	portFlag             = flag.Int("port", 9002, "port to run app server")
	metricsPortFlag      = flag.Int("metrics_port", 9012, "port to run metrics server")
	replicaIDFlag        = flag.String("hostname", uuid.NewString(), "unique id of service")
	helmChartVersionFlag = flag.String("chart_version", "", "")
	tagVersionFlag       = flag.String("tag_version", "", "")
	appNameFlag          = flag.String("app_name", "ioc-cfn-svc", "")
	cfnIDFlag            = flag.String("cfn_id", uuid.NewString(), "unique persistent CFN identifier")

	oomGracefulExitThresholdFlag = flag.Float64("oom_graceful_exit_threshold", 0.98, "")

	// database configs
	dbHostFlag        = flag.String("db_host", "", "")
	dbPortFlag        = flag.String("db_port", "", "")
	dbNameFlag        = flag.String("db_name", "", "")
	dbUserFlag        = flag.String("db_user", "", "")
	dbPasswordFlag    = flag.String("db_password", "", "")
	dbSslModeFlag     = flag.String("db_ssl_mode", "", "")
	dbSslRootCertFlag = flag.String("db_ssl_root_cert", "", "")

	externalServiceURLFlag = flag.String("external_service_url", "", "")

	idpLabelFlag                     = flag.String("idp_label", "", "label for the idp")
	idpClientIDFlag                  = flag.String("idp_client_id", "", "client id for the idp")
	idpClientSecretFlag              = flag.String("idp_client_secret", "", "client secret for the idp")
	idpIssuerFlag                    = flag.String("idp_issuer", "", "issuer of the idp")
	idpAudienceFlag                  = flag.String("idp_audience", "", "audience of the idp")
	idpDefaultLoginCallbackPathFlag  = flag.String("idp_default_login_callback_path", "", "the default login redirect path for the idp")
	idpDefaultSignupCallbackPathFlag = flag.String("idp_default_signup_callback_path", "", "the default signup redirect path for the idp")
	idpIssuerLogoutPathFlag          = flag.String("idp_issuer_logout_path", "", "issuer logout path for the idp")
	idpSignupURLFlag                 = flag.String("idp_signup_url", "", "url where the user can signup for an account with the idp")
)

type Config struct {
	AppPort                  int
	MetricsPort              int
	HostID                   string
	HelmChartVersion         string
	TagVersion               string
	ServiceName              string
	OomGracefulExitThreshold float64
	ExternalServiceURL       string
	CfnID                    string

	DB  Database
	IDP IdentityProvider
}

func Get() *Config {
	flag.Parse()
	return &Config{
		AppPort:                  *portFlag,
		MetricsPort:              *metricsPortFlag,
		HostID:                   *replicaIDFlag,
		HelmChartVersion:         *helmChartVersionFlag,
		TagVersion:               *tagVersionFlag,
		ServiceName:              *appNameFlag,
		OomGracefulExitThreshold: *oomGracefulExitThresholdFlag,
		ExternalServiceURL:       *externalServiceURLFlag,
		CfnID:                    *cfnIDFlag,
		DB: Database{
			Host:        *dbHostFlag,
			Port:        *dbPortFlag,
			Name:        *dbNameFlag,
			User:        *dbUserFlag,
			Password:    *dbPasswordFlag,
			SSLMode:     *dbSslModeFlag,
			SSLRootCert: *dbSslRootCertFlag,
		},
		IDP: IdentityProvider{
			Label:                     *idpLabelFlag,
			ClientID:                  *idpClientIDFlag,
			ClientSecret:              *idpClientSecretFlag,
			Issuer:                    *idpIssuerFlag,
			Audience:                  *idpAudienceFlag,
			DefaultLoginCallbackPath:  *idpDefaultLoginCallbackPathFlag,
			DefaultSignupCallbackPath: *idpDefaultSignupCallbackPathFlag,
			IssuerLogoutPath:          *idpIssuerLogoutPathFlag,
			SignupURL:                 *idpSignupURLFlag,
		},
	}
}

func Log() {
	flag.Parse()
	flag.VisitAll(func(f *flag.Flag) {
		if containsAny(f.Name, "secret", "password", "pswd", "admin", "api_key") {
			// don't log secrets
			log.Infof("%40v : %d", "len("+f.Name+")", len(f.Value.String()))
		} else {
			log.Infof("%40v : %s", f.Name, f.Value)
		}
	})
}

func containsAny(str string, substr ...string) bool {
	s := strings.ToLower(str)
	for _, sub := range substr {
		if strings.Contains(s, sub) {
			return true
		}
	}

	return false
}

type Database struct {
	Host        string
	Port        string
	Name        string
	User        string
	Password    string
	SSLMode     string
	SSLRootCert string
}

func (d Database) sslmode() string {
	if d.SSLMode != "" {
		return fmt.Sprintf("sslmode=%s", d.SSLMode)
	}
	//nolint:goconst // allow "localhost" string
	if d.Host == "localhost" || !strings.Contains(d.Host, ".") {
		return "sslmode=disable"
	}
	return "sslmode=require"
}

func (d Database) sslrootcert() string {
	if d.SSLRootCert != "" {
		return fmt.Sprintf("sslrootcert=%s", d.SSLRootCert)
	}
	return ""
}

func (d Database) DSN() string {
	return strings.TrimSpace(fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s timezone=%s %s %s",
		d.Host,
		d.Port,
		d.Name,
		d.User,
		d.Password,
		"UTC",
		d.sslmode(),
		d.sslrootcert(),
	))
}

func (d Database) Copy() Database {
	return Database{
		Host:        d.Host,
		Port:        d.Port,
		Name:        d.Name,
		User:        d.User,
		Password:    d.Password,
		SSLMode:     d.SSLMode,
		SSLRootCert: d.SSLRootCert,
	}
}

func (d Database) RawPostgresURL(extraPairs ...string) string {
	cleaned := []string{}
	for _, extraPair := range extraPairs {
		ep := strings.TrimSpace(extraPair)
		if ep != "" {
			parts := strings.Split(ep, "=")
			if len(parts) == 2 {
				cleaned = append(cleaned, fmt.Sprintf("%s=%s",
					parts[0], url.QueryEscape(parts[1])))
			}
		}
	}
	suffix := strings.Join(cleaned, "&")
	if len(suffix) > 0 {
		suffix = "?" + suffix
	}

	return fmt.Sprintf(
		"postgresql://%s:%s@%s:%s/%s%s",
		d.User,
		url.QueryEscape(d.Password),
		d.Host,
		d.Port,
		d.Name,
		suffix)
}

func (d Database) PostgresURL(extraPairs ...string) string {
	eps := []string{d.sslmode(), d.sslrootcert(), "timezone=UTC"}
	eps = append(eps, extraPairs...)
	return d.RawPostgresURL(eps...)
}

func (d Database) PostgresMigrateURL() string {
	return d.PostgresURL("search_path=public")
}

func (d Database) Loggable() string {
	d.Password = fmt.Sprintf("<redacted_%d>", len(d.Password))
	return d.DSN()
}

func (d Database) Enabled() bool {
	return d.Host != "" && d.Port != "" && d.Name != "" && d.User != "" &&
		d.Password != ""
}

type IdentityProvider struct {
	Label                     string
	ClientID                  string
	ClientSecret              string
	Issuer                    string
	Audience                  string
	DefaultLoginCallbackPath  string
	DefaultSignupCallbackPath string
	IssuerLogoutPath          string
	SignupURL                 string
}

func (idp IdentityProvider) Loggable() string {
	if idp.Issuer != "" && idp.Audience != "" {
		return fmt.Sprintf("label=%s client_id=%s issuer=%s audience=%s",
			idp.Label, idp.ClientID, idp.Issuer, idp.Audience)
	}
	return fmt.Sprintf("label=%s client_id=%s", idp.Label, idp.ClientID)
}

func (idp IdentityProvider) Enabled() bool {
	return idp.Label != "" && idp.ClientID != ""
}
