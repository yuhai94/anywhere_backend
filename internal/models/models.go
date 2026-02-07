package models

import (
	"time"
)

type V2RayInstance struct {
	ID          int       `db:"id" json:"id"`
	UUID        string    `db:"uuid" json:"uuid"`
	EC2ID       string    `db:"ec2_id" json:"ec2_id"`
	EC2Region   string    `db:"ec2_region" json:"ec2_region"`
	EC2PublicIP string    `db:"ec2_public_ip" json:"ec2_public_ip"`
	Status      string    `db:"status" json:"status"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
	IsDeleted   bool      `db:"is_deleted" json:"-"`
}

const (
	StatusPending  = "pending"
	StatusCreating = "creating"
	StatusRunning  = "running"
	StatusDeleting = "deleting"
	StatusDeleted  = "deleted"
	StatusError    = "error"
)
