package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
)

func main() {
	repo := os.Getenv("RESTIC_REPOSITORY")

	cfg, err := parseConfig(repo)
	if err != nil {
		log.Fatal(err)
	}

	client, err := newClient(cfg)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	_, _, _, err = client.GetBucketObjectLockConfig(ctx, cfg.Bucket)
	if err != nil {
		log.Fatalf("you need a bucket with object lock configuration enabled at creation time: %s", err)
	}

	binary := os.Getenv("RPS_RESTIC_COMMAND")
	if binary == "" {
		binary = "restic"
	}

	out, err := exec.Command(
		binary,
		"--json",
		"--quiet",
		"restore",
		"--dry-run",
		"--target", "~/doesnotmatter",
		"latest").Output()
	if err != nil {
		log.Fatal(err)
	}

	rdr := RestoreDryRun{}
	err = json.Unmarshal(out, &rdr)
	if err != nil {
		log.Fatal(err)
	}

	files := []string{
		fmt.Sprintf("%s/snapshots/%s", cfg.Prefix, rdr.Snapshot),
		fmt.Sprintf("%s/config", cfg.Prefix),
	}

	dir := "keys/"
	if cfg.Prefix != "" {
		dir = fmt.Sprintf("%s/keys", cfg.Prefix)
	}
	for obj := range client.ListObjects(ctx, cfg.Bucket, minio.ListObjectsOptions{
		Prefix:    dir,
		Recursive: false,
	}) {
		if obj.Err != nil {
			log.Fatal(err)
		}

		if obj.Key == "" {
			continue
		}

		name := strings.TrimPrefix(obj.Key, dir)
		// Sometimes s3 returns an entry for the dir itself. Ignore it.
		if name == "" {
			continue
		}
		files = append(files, obj.Key)
	}

	for _, pname := range rdr.Packs {
		fname := fmt.Sprintf("%s/data/%s/%s", cfg.Prefix, pname[0:2], pname)
		files = append(files, fname)
	}

	then := time.Now().Add(25 * time.Hour)

	g := minio.Governance

	for _, fname := range files {
		if strings.HasPrefix(fname, "/") {
			fname = fname[1:]
		}
		err = client.PutObjectRetention(ctx, cfg.Bucket, fname, minio.PutObjectRetentionOptions{
			GovernanceBypass: false,
			Mode:             &g,
			RetainUntilDate:  &then,
		})
		if err != nil {
			log.Fatal(err)
		}

		mode, until, err := client.GetObjectRetention(ctx, cfg.Bucket, fname, "")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%#v %v %v\n", fname, mode, until)
	}

}

type RestoreDryRun struct {
	Snapshot string   `json:"snapshot"`
	Packs    []string `json:"packs"`
}

func newClient(cfg *Config) (*minio.Client, error) {
	creds := credentials.NewChainCredentials([]credentials.Provider{
		&credentials.EnvAWS{},
		&credentials.EnvMinio{},
		&credentials.FileAWSCredentials{},
		&credentials.FileMinioClient{},
		&credentials.IAM{
			Client: &http.Client{
				Transport: http.DefaultTransport,
			},
		},
	})

	options := &minio.Options{
		Creds:     creds,
		Region:    cfg.Region,
		Secure:    !cfg.UseHTTP,
		Transport: http.DefaultTransport,
	}

	client, err := minio.New(cfg.Endpoint, options)
	if err != nil {
		return nil, errors.Wrap(err, "minio.New")
	}

	return client, nil
}
