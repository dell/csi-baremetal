package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/scheduler"
)

const defaultSyncInterval = 60

var (
	manifestPath     = flag.String("manifest", "", "path to the scheduler manifest file")
	sourceConfigPath = flag.String("source-config-path", "",
		"source path for scheduler config file")
	sourcePolicyPath = flag.String("source-policy-path", "",
		"source path for scheduler policy file")
	targetConfigPath = flag.String("target-config-path", "",
		"target path for scheduler config file")
	targetPolicyPath = flag.String("target-policy-path", "",
		"target path for scheduler policy file")
	backupPath = flag.String("backup-path", "",
		"path to store manifest backup")
	syncInterval = flag.Int("interval", defaultSyncInterval,
		fmt.Sprintf("interval to check manifest config, default: %d", defaultSyncInterval))
	restoreOnShutdown = flag.Bool("restore", false, "restore manifest when on shutdown")
	logLevel          = flag.String("loglevel", base.InfoLevel, "Log level")
)

func main() {
	flag.Parse()
	logger, _ := base.InitLogger("", *logLevel)
	logger.Info("Starting scheduler patcher ...")

	patcher := scheduler.NewManifestPatcher(logger)

	ticker := time.NewTicker(time.Second * time.Duration(*syncInterval))
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for _, opt := range []*string{manifestPath, sourceConfigPath, sourcePolicyPath,
		targetConfigPath, targetPolicyPath, backupPath} {
		if *opt == "" {
			flag.Usage()
			os.Exit(1)
		}
	}

	config := scheduler.ManifestPatcherConfig{
		ManifestPath:     *manifestPath,
		SourceConfigPath: *sourceConfigPath,
		TargetConfigPath: *targetConfigPath,
		SourcePolicyPath: *sourcePolicyPath,
		TargetPolicyPath: *targetPolicyPath,
		BackupPath:       *backupPath,
	}

	apply := func() {
		err := patcher.Apply(config)
		if err != nil {
			logger.Error(err.Error())
		}
	}

	apply()

Loop:
	for {
		select {
		case <-sigs:
			logger.Info("shutdown")
			if *restoreOnShutdown {
				_ = patcher.Restore(config)
			}
			break Loop
		case <-ticker.C:
			apply()
		}
	}

	os.Exit(0)
}
