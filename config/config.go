package config

const (
	ServerName                 = "server.name"
	ServerMode                 = "server.mode"
	ServerHttpPort             = "server.http_port"
	ServerEnableTLS            = "server.enable_tls"
	ServerCertFile             = "server.cert_file"
	ServerKeyFile              = "server.key_file"
	ServerGracefulShutdownWait = "server.graceful_shutdown_wait"
	ServerJWTPrivateKey        = "server.jwt_private_key"
	ServerJWTPublicKey         = "server.jwt_public_key"
	ServerRefreshPrivateKey    = "server.refresh_private_key"
	ServerRefreshPublicKey     = "server.refresh_public_key"
	ServerSuperAdminPassword   = "server.superadmin_password"
	ServerTestMode             = "server.test_mode"
	ServerValidRedirect        = "server.valid_redirect"
	ServerDefaultRedirect      = "server.default_redirect"
	ServerCookieDomain         = "server.cookie_domain"
)

const (
	APITimeoutDuration = "api.timeout_duration"
)

const (
	TracerURI = "tracer.uri"
)

const (
	DatabaseDriver          = "database.driver"
	DatabaseHost            = "database.host"
	DatabasePort            = "database.port"
	DatabaseUser            = "database.user"
	DatabasePassword        = "database.password"
	DatabaseName            = "database.name"
	DatabaseIsSecure        = "database.is_secure"
	DatabaseMaxOpenConn     = "database.max_open_conn"
	DatabaseMaxIdleConn     = "database.max_idle_conn"
	DatabaseConnMaxLifetime = "database.conn_max_lifetime"
	DatabaseConnMaxIdleTime = "database.conn_max_idle_time"
	DatabaseReadTimeout     = "database.read_timeout"
	DatabaseWriteTimeout    = "database.write_timeout"
	DatabaseTimeout         = "database.timeout"
)

const (
	MqttURI                     = "mqtt.uri"
	MqttCleanSession            = "mqtt.clean_session"
	MqttClientId                = "mqtt.client_id"
	MqttAutoReconnect           = "mqtt.auto_reconnect"
	MqttConnectRetry            = "mqtt.connect_retry"
	MqttMaxConnectInterval      = "mqtt.max_connect_interval"
	MqttWriteTimeout            = "mqtt.write_timeout"
	MqttPingTimeout             = "mqtt.ping_timeout"
	MqttKeepAliveDuration       = "mqtt.keep_alive_duration"
	MqttQOS                     = "mqtt.qos"
	MqttTopic                   = "mqtt.topic"
	MqttDataCaptureTopic        = "mqtt.data_capture_topic"
	MqttDataCaptureStatusTopic  = "mqtt.data_capture_status_topic"
	MqttRosTopicListTopic       = "mqtt.ros_topic_list_topic"
	MqttRosTopicListResultTopic = "mqtt.ros_topic_list_result_topic"
	MqttRobotHealthcheckTopic   = "mqtt.robot_healthcheck_topic"
)

const (
	S3URI       = "s3.uri"
	S3AccessKey = "s3.access_key"
	S3SecretKey = "s3.secret_key" // #nosec G101
	S3Bucket    = "s3.bucket"
	S3Region    = "s3.region"
	S3IsSecure  = "s3.is_secure"
)

const (
	RedisHost     = "redis.host"
	RedisPassword = "redis.password"
)

const (
	AirflowHost            = "airflow.host"
	AirflowScheme          = "airflow.scheme"
	AirflowUsername        = "airflow.username"
	AirflowPassword        = "airflow.password"
	AirflowRetargetDAGName = "airflow.retarget_dag_name"
)

const (
	CentrifugoHost = "centrifugo.host"
)

const (
	KeycloakOIDCProvider  = "keycloak.oidc_provider"
	KeycloakHost          = "keycloak.host"
	KeycloakClientID      = "keycloak.client_id"
	KeycloakClientSecret  = "keycloak.client_secret"
	KeycloakAdminRealm    = "keycloak.admin_realm"
	KeycloakRealm         = "keycloak.realm"
	KeycloakRedirectURL   = "keycloak.redirect_url"
	KeycloakAdminUsername = "keycloak.admin_username"
	KeycloakAdminPassword = "keycloak.admin_password"
)

const (
	OIDCConfigURL    = "oidc.config_url"
	OIDCClientID     = "oidc.client_id"
	OIDCClientSecret = "oidc.client_secret"
	OIDCDisplayName  = "oidc.display_name"
	OIDCScopes       = "oidc.scopes"
	OIDCRedirectURI  = "oidc.redirect_uri"
	OIDCRealm        = "oidc.realm"
)
