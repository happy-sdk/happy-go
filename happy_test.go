// SPDX-License-Identifier: Apache-2.0
//
// Copyright © 2022 The Happy Authors

package happy_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/happy-sdk/happy"
	"github.com/happy-sdk/happy/pkg/devel/testutils"
	"github.com/happy-sdk/happy/sdk/logging"
)

func TestDefaultInfo(t *testing.T) {
	log := logging.NewTestLogger(logging.LevelError)

	app := happy.New(happy.Settings{})
	app.WithLogger(log)
	app.Do(func(sess *happy.Session, args happy.Args) error {
		testutils.Equal(t, "Happy Prototype", sess.Get("app.name").String(), "app.name")
		testutils.Equal(t, "com.github.happy-sdk.happy-test", sess.Get("app.slug").String(), "app.slug")
		testutils.Equal(t, "This application is built using the Happy-SDK to provide enhanced functionality and features.", sess.Get("app.description").String(), "app.description")
		testutils.Equal(t, "Anonymous", sess.Get("app.copyright_by").String(), "app.copyright_by")
		testutils.Equal(t, time.Now().Year(), sess.Get("app.copyright_since").Int(), "app.copyright_since")
		testutils.Equal(t, "NOASSERTION", sess.Get("app.license").String(), "app.license")
		testutils.Equal(t, "com.github.happy-sdk.happy.test", sess.Get("app.identifier").String(), "app.identifier")
		return nil
	})

	app.Run()
}

func TestDefaultSettings(t *testing.T) {
	log := logging.NewTestLogger(logging.LevelError)

	app := happy.New(happy.Settings{})
	app.WithLogger(log)
	app.Do(func(sess *happy.Session, args happy.Args) error {
		// CLI
		testutils.Equal(t, "0", sess.Get("app.cli.argn_max").String(), "app.cli.argn_max")
		// DateTime
		testutils.Equal(t, "Local", sess.Get("app.datetime.location").String(), "app.datetime.location")
		// Instance
		testutils.Equal(t, "1s", sess.Get("app.instance.throttle_ticks").String(), "app.instance.throttle_ticks")
		testutils.Equal(t, time.Second*1, sess.Get("app.instance.throttle_ticks").Duration(), "app.instance.throttle_ticks")
		testutils.Equal(t, "1", sess.Get("app.instance.max").String(), "app.instance.max")
		testutils.Equal(t, 1, sess.Get("app.instance.max").Int(), "app.instance.max")
		// Logging

		// Services
		testutils.Equal(t, "30s", sess.Get("app.services.loader_timeout").String(), "app.services.loader_timeout")
		testutils.Equal(t, time.Second*30, sess.Get("app.services.loader_timeout").Duration(), "app.services.loader_timeout")
		// Stats
		testutils.Equal(t, false, sess.Get("app.stats.enabled").Bool(), "app.stats.enabled")

		return nil
	})

	app.Run()
}

func TestDefaultConfig(t *testing.T) {
	log := logging.NewTestLogger(logging.LevelError)
	app := happy.New(happy.Settings{})
	app.WithLogger(log)
	app.Do(func(sess *happy.Session, args happy.Args) error {
		testutils.Equal(t, 15, sess.Opts().Len(), "invalid default runtime config key count")
		host, err := os.Hostname()
		if err != nil {
			return err
		}
		addr := fmt.Sprintf("happy://%s/com.github.happy-sdk.happy-test", host)
		testutils.Equal(t, addr, sess.Get("app.address").String(), "app.address")
		testutils.Equal(t, true, sess.Get("app.devel").Bool(), "app.devel")
		testutils.Equal(t, true, sess.Get("app.firstuse").Bool(), "app.firstuse")
		testutils.Equal(t, false, sess.Get("app.main.exec.x").Bool(), "app.main.exec.x")
		testutils.Equal(t, "github.com/happy-sdk/happy", sess.Get("app.module").String(), "app.module")
		testutils.Equal(t, "public-devel", sess.Get("app.profile.name").String(), "app.profile.name")
		testutils.Equal(t, "v1.0.0-0xDEV", sess.Get("app.version").String(), "app.version")

		tmpdir := sess.Get("app.fs.path.tmp").String()
		testutils.HasPrefix(t, tmpdir, os.TempDir(), "app.fs.path.tmp")

		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		testutils.Equal(t, wd, sess.Get("app.fs.path.pwd").String(), "app.fs.path.pwd")

		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		testutils.Equal(t, home, sess.Get("app.fs.path.home").String(), "app.fs.path.home")

		testutils.Equal(t, fmt.Sprintf("%s/config/com.github.happy-sdk.happy-test/profiles/public-devel", tmpdir), sess.Get("app.fs.path.config").String(), "app.fs.path.config")
		testutils.Equal(t, fmt.Sprintf("%s/cache/com.github.happy-sdk.happy-test/profiles/public-devel", tmpdir), sess.Get("app.fs.path.cache").String(), "app.fs.path.cache")
		testutils.Equal(t, fmt.Sprintf("%s/config/com.github.happy-sdk.happy-test/profiles/public-devel/profile.preferences", tmpdir), sess.Get("app.profile.preferences").String(), "app.profile.preferences")
		testutils.Equal(t, fmt.Sprintf("%s/config/com.github.happy-sdk.happy-test/profiles/public-devel/pids", tmpdir), sess.Get("app.fs.path.pids").String(), "app.fs.path.pids")
		testutils.Equal(t, os.Getpid(), sess.Get("app.pid").Int(), "app.pid")

		return nil
	})
	app.Run()
}
