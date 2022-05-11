package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/novikovoleg142/vault_raft_snapshot_agent/config"
	"github.com/novikovoleg142/vault_raft_snapshot_agent/crypto"

	"github.com/novikovoleg142/vault_raft_snapshot_agent/snapshot_agent"
)

func listenForInterruptSignals() chan bool {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool, 1)

	go func() {
		_ = <-sigs
		done <- true
	}()
	return done
}

func main() {
	done := listenForInterruptSignals()

	log.Println("Reading configuration...")
	c, err := config.ReadConfig()

	if err != nil {
		log.Fatalln("Configuration could not be found")
	}

	snapshotter, err := snapshot_agent.NewSnapshotter(c)
	if err != nil {
		log.Fatalln("Cannot instantiate snapshotter.", err)
	}
	frequency, err := time.ParseDuration(c.Frequency)

	if err != nil {
		frequency = time.Hour
		log.Println("Cannot parse time frequency, set 1 hour")
	}

	for {
		if snapshotter.TokenExpiration.Before(time.Now()) {
			switch c.VaultAuthMethod {
			case "k8s":
				log.Println("Set k8s auth")
				snapshotter.SetClientTokenFromK8sAuth(c)
			default:
				log.Println("Set Approle auth")
				snapshotter.SetClientTokenFromAppRole(c)
			}
		}
		leader, err := snapshotter.API.Sys().Leader()
		if err != nil {
			log.Println(err.Error())
			log.Fatalln("Unable to determine leader instance.  The snapshot agent will only run on the leader node.  Are you running this daemon on a Vault instance?")
		}
		leaderIsSelf := leader.IsSelf
		if !leaderIsSelf {
			log.Println("Not running on leader node, skipping.")
		} else {
			var snapshot bytes.Buffer
			err := snapshotter.API.Sys().RaftSnapshot(&snapshot)
			if err != nil {
				log.Fatalln("Unable to generate snapshot", err.Error())
			}
			now := time.Now().UnixNano()
			if c.AWS.Bucket != "" {
				if c.Encrypt_pass != "" {
					log.Println("Encrypting backup file...")
					crypto, err := crypto.NewCrypto([]byte(c.Encrypt_pass))
					if err != nil {
						log.Fatalln("Unable to create crypt instance", err.Error())
					}
					var b bytes.Buffer
					err = crypto.Encrypt(&snapshot, &b)
					if err != nil {
						log.Fatalln("Unable to crypt snapshot", err.Error())
					}
					var w []byte
					w = b.Bytes()
					err = os.WriteFile("/tmp/dat1.snap.gpg", w, 0644)
					if err != nil {
						fmt.Println("Unable to write temp crypt file:", err)
					}
					file, err := os.Open("/tmp/dat1.snap.gpg")
					if err != nil {
						fmt.Println("Unable to open temp crypt file:", err)
						os.Exit(1)
					}
					defer file.Close()
					var r io.ReadWriter
					r = file
					snapshotPath, err := snapshotter.CreateS3Snapshot(r, c, now, true)
					logSnapshotError("aws", snapshotPath, err)
				} else {
					snapshotPath, err := snapshotter.CreateS3Snapshot(&snapshot, c, now, false)
					logSnapshotError("aws", snapshotPath, err)
				}
			} else {
				log.Fatalln("Unable to find S3 bucket name, please set it in config. Terminating...")
			}
		}
		select {
		case <-time.After(frequency):
			continue
		case <-done:
			os.Exit(1)
		}
	}
}

func logSnapshotError(dest, snapshotPath string, err error) {
	if err != nil {
		log.Printf("Failed to generate %s snapshot to %s: %v\n", dest, snapshotPath, err)
	} else {
		log.Printf("Successfully created %s snapshot to %s\n", dest, snapshotPath)
	}
}
