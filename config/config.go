package config

// 全局运行时配置变量。
// 这些值可在程序启动时通过命令行参数、环境变量或配置文件进行设置。
var (
	DbPath         string // 数据库文件路径（运行时覆盖 DbFile）
	LogLevel       string // 日志级别
	QiniuAccessKey string // 七牛云 Access Key
	QiniuSecretKey string // 七牛云 Secret Key
	QiniuBucket    string // 七牛云存储空间名
	PasswordHash   string // 密码哈希值（用于本地加密验证）
)

// SetLogPath 设置日志文件路径（覆盖默认值）。
func SetLogPath(path string) {
	LogPath = path
}

// GetLogPath 返回日志文件路径，如果未设置则返回默认值 "/tmp/assh.log"。
func GetLogPath() string {
	if LogPath != "" {
		return LogPath
	}
	return "/tmp/assh.log"
}

// SetDbPath 设置数据库文件路径（覆盖默认值）。
func SetDbPath(path string) {
	DbPath = path
}

// GetDbPath 返回数据库文件路径，如果未设置则返回默认 DbFile 值。
func GetDbPath() string {
	if DbPath != "" {
		return DbPath
	}
	return DbFile
}

// SetLogLevel 设置日志级别。
func SetLogLevel(level string) {
	LogLevel = level
}

// GetLogLevel 返回当前日志级别。
func GetLogLevel() string {
	return LogLevel
}

// SetQiniuConfig 设置七牛云存储的认证信息和存储空间。
func SetQiniuConfig(accessKey, secretKey, bucket string) {
	QiniuAccessKey = accessKey
	QiniuSecretKey = secretKey
	QiniuBucket = bucket
}

// GetQiniuConfig 返回七牛云存储的认证信息和存储空间。
func GetQiniuConfig() (accessKey, secretKey, bucket string) {
	return QiniuAccessKey, QiniuSecretKey, QiniuBucket
}

// HasPassword 检查是否已设置密码哈希。
func HasPassword() bool {
	return PasswordHash != ""
}

// SetPasswordHash 设置密码哈希值。
func SetPasswordHash(hash string) {
	PasswordHash = hash
}

// GetPasswordHash 返回密码哈希值。
func GetPasswordHash() string {
	return PasswordHash
}