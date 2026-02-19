package models

import (
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

type CustomTime struct {
	time.Time
}

const customTimeFormat = "2006-01-02 15:04:05"

func (ct CustomTime) MarshalJSON() ([]byte, error) {
	formatted := fmt.Sprintf("\"%s\"", ct.Time.Format(customTimeFormat))
	return []byte(formatted), nil
}

func (ct *CustomTime) UnmarshalJSON(b []byte) error {
	str := string(b)
	str = str[1 : len(str)-1]
	t, err := time.Parse(customTimeFormat, str)
	if err != nil {
		return err
	}
	ct.Time = t
	return nil
}

func (ct CustomTime) Value() (driver.Value, error) {
	return ct.Time, nil
}

func (ct *CustomTime) Scan(value interface{}) error {
	if t, ok := value.(time.Time); ok {
		ct.Time = t
		return nil
	}
	return fmt.Errorf("cannot scan %v into CustomTime", value)
}

type V2RayInstance struct {
	ID            int        `db:"id" json:"id"`
	UUID          string     `db:"uuid" json:"uuid"`
	EC2ID         string     `db:"ec2_id" json:"ec2_id"`
	EC2Region     string     `db:"ec2_region" json:"ec2_region"`
	EC2RegionName string     `db:"-" json:"ec2_region_name"`
	EC2PublicIP   string     `db:"ec2_public_ip" json:"ec2_public_ip"`
	Status        string     `db:"status" json:"status"`
	DirectLink    string     `db:"direct_link" json:"direct_link"`
	RelayLink     string     `db:"relay_link" json:"relay_link"`
	CreatedAt     CustomTime `db:"created_at" json:"created_at"`
	UpdatedAt     CustomTime `db:"updated_at" json:"updated_at"`
	IsDeleted     bool       `db:"is_deleted" json:"-"`
}

type Region struct {
	Region string `json:"region"`
	Name   string `json:"name"`
}

const (
	StatusPending  = "pending"
	StatusCreating = "creating"
	StatusRunning  = "running"
	StatusDeleting = "deleting"
	StatusDeleted  = "deleted"
	StatusError    = "error"
)

type VMessConfig struct {
	Add  string `json:"add"`
	Aid  string `json:"aid"`
	Alpn string `json:"alpn"`
	Fp   string `json:"fp"`
	Host string `json:"host"`
	ID   string `json:"id"`
	Net  string `json:"net"`
	Path string `json:"path"`
	Port string `json:"port"`
	Ps   string `json:"ps"`
	Scy  string `json:"scy"`
	Sni  string `json:"sni"`
	Tls  string `json:"tls"`
	Type string `json:"type"`
	V    string `json:"v"`
}

func GenerateVMessLink(add, id, port, ps string) (string, error) {
	config := VMessConfig{
		Add:  add,
		Aid:  "0",
		Alpn: "",
		Fp:   "",
		Host: "",
		ID:   id,
		Net:  "tcp",
		Path: "",
		Port: port,
		Ps:   ps,
		Scy:  "auto",
		Sni:  "",
		Tls:  "",
		Type: "none",
		V:    "2",
	}

	jsonData, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal vmess config: %v", err)
	}

	base64Data := base64.StdEncoding.EncodeToString(jsonData)
	return "vmess://" + base64Data, nil
}
