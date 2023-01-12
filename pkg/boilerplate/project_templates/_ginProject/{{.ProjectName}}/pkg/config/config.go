//go:build exclude || ignore
// +build exclude ignore

package config

type Config struct {
	Addr string `json:"addr"`
	Port int    `json:"port"`
}
