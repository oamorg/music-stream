package store

import (
	"context"
	"database/sql"
	"net"
	"net/url"
	"strings"
	"time"
)

type ReadinessChecker struct {
	db            *sql.DB
	redisAddr     string
	minioEndpoint string
}

func NewReadinessChecker(db *sql.DB, redisAddr, minioEndpoint string) ReadinessChecker {
	return ReadinessChecker{
		db:            db,
		redisAddr:     redisAddr,
		minioEndpoint: minioEndpoint,
	}
}

func (c ReadinessChecker) Check(ctx context.Context) (bool, map[string]bool) {
	checks := map[string]bool{
		"database": c.pingDatabase(ctx),
		"redis":    pingTCP(ctx, c.redisAddr),
		"minio":    pingTCP(ctx, normalizeTCPAddress(c.minioEndpoint)),
	}

	for _, ok := range checks {
		if !ok {
			return false, checks
		}
	}

	return true, checks
}

func (c ReadinessChecker) pingDatabase(ctx context.Context) bool {
	if c.db == nil {
		return false
	}

	checkCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	return c.db.PingContext(checkCtx) == nil
}

func pingTCP(ctx context.Context, address string) bool {
	address = strings.TrimSpace(address)
	if address == "" {
		return false
	}

	checkCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	var dialer net.Dialer
	conn, err := dialer.DialContext(checkCtx, "tcp", address)
	if err != nil {
		return false
	}

	_ = conn.Close()
	return true
}

func normalizeTCPAddress(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err == nil && parsed.Host != "" {
			return parsed.Host
		}
	}

	return raw
}
