package containers

const (
	CouchDB       = "couchdb"
	ElasticSearch = "elasticsearch"
	Kafka         = "kafka"
	Keycloak      = "keycloak"
	InfluxDB      = "influxdb"
	MariaDB       = "mariadb"
	Memcached     = "memcached"
	MilvusDB      = "milvusdb"
	MinIO         = "minio"
	MongoDB       = "mongodb"
	MySQL         = "mysql"
	PostgreSQL    = "postgresql"
	RabbitMQ      = "rabbitmq"
	Redis         = "redis"
)

var Mapper = map[string]bool{
	Redis:         true,
	Keycloak:      true,
	MinIO:         true,
	PostgreSQL:    true,
	MySQL:         true,
	MariaDB:       true,
	MongoDB:       true,
	Memcached:     true,
	RabbitMQ:      true,
	InfluxDB:      true,
	ElasticSearch: true,
	CouchDB:       true,
	MilvusDB:      true,
	Kafka:         true,
}
