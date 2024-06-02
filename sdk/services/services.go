// SPDX-License-Identifier: Apache-2.0
//
// Copyright © 2024 The Happy Authors

package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/happy-sdk/happy/pkg/scheduling/cron"
	"github.com/happy-sdk/happy/pkg/settings"
	"github.com/happy-sdk/happy/pkg/vars"
	"github.com/happy-sdk/happy/sdk/action"
	"github.com/happy-sdk/happy/sdk/app/session"
	"github.com/happy-sdk/happy/sdk/events"
	"github.com/happy-sdk/happy/sdk/internal"
	"github.com/happy-sdk/happy/sdk/networking/address"
	"github.com/happy-sdk/happy/sdk/services/service"
)

var (
	Error = fmt.Errorf("services error")
	// StartEvent starts services defined in payload
	StartEvent = events.New("services", "start.services")
)

type Settings struct {
	LoaderTimeout  settings.Duration `key:"loader_timeout,save" default:"30s" mutation:"once" desc:"Service loader timeout"`
	RunCronOnStart settings.Bool     `key:"cron_on_service_start,save" default:"false" mutation:"once" desc:"Run cron jobs on service start"`
}

func (s Settings) Blueprint() (*settings.Blueprint, error) {
	b, err := settings.New(s)
	if err != nil {
		return nil, err
	}

	return b, nil
}

type Info interface {
	Running() bool
	Name() string
	StartedAt() time.Time
	StoppedAt() time.Time
	Addr() *address.Address
	Failed() bool
	Errs() map[time.Time]error
}

type ServiceLoader struct {
	loading  bool
	loaderCh chan struct{}
	errs     []error
	sess     *session.Context
	hostaddr *address.Address
	svcs     []*address.Address
}

// NewServiceLoader creates new service loader which can be used to load services.
func NewLoader(sess *session.Context, svcs ...string) *ServiceLoader {
	loader := &ServiceLoader{
		sess:     sess,
		loaderCh: make(chan struct{}),
	}
	hostaddr, err := address.Parse(sess.Get("app.address").String())
	if err != nil {
		loader.addErr(err)
		loader.addErr(fmt.Errorf(
			"%w: loader requires valid app.address",
			Error,
		))
	}

	loader.hostaddr = hostaddr
	for _, addr := range svcs {
		svcaddr, err := hostaddr.ResolveService(addr)

		if err != nil {
			loader.addErr(err)
		} else {
			loader.svcs = append(loader.svcs, svcaddr)
		}
	}

	return loader
}

func (sl *ServiceLoader) Load() <-chan struct{} {
	if sl.loading {
		return sl.loaderCh
	}
	sl.loading = true
	if len(sl.errs) > 0 {
		sl.cancel(fmt.Errorf(
			"%w: loader initializeton failed",
			Error,
		))
		return sl.loaderCh
	}
	timeout := sl.sess.Get("app.services.loader_timeout").Duration()
	if timeout <= 0 {
		timeout = time.Duration(time.Second * 30)
		sl.sess.Log().NotImplemented(
			"service loader using default timeout",
			slog.Duration("timeout", timeout),
			slog.Duration("app.services.loader_timeout", timeout),
		)
	}
	internal.Log(sl.sess.Log(), "loading services", slog.String("host", sl.hostaddr.Host()), slog.String("instance", sl.hostaddr.Instance()))

	queue := make(map[string]*service.Info)
	var require []string

	for _, svcaddr := range sl.svcs {
		svcaddrstr := svcaddr.String()
		info, err := sl.sess.ServiceInfo(svcaddrstr)
		if err != nil {
			sl.cancel(err)
			return sl.loaderCh
		}
		if _, ok := queue[svcaddrstr]; ok {
			sl.cancel(fmt.Errorf(
				"%w: duplicated service request %s",
				Error,
				svcaddrstr,
			))
			return sl.loaderCh
		}
		if info.Running() {
			sl.sess.Log().NotImplemented(
				"requested service is already running",
				slog.String("service", svcaddrstr),
			)
			continue
		}
		internal.Log(sl.sess.Log(), "requesting service", slog.String("service", svcaddrstr))
		queue[svcaddrstr] = info
		require = append(require, svcaddrstr)
	}

	sl.sess.Dispatch(startEvent(require...))

	ctx, cancel := context.WithTimeout(sl.sess, timeout)

	go func() {
		defer cancel()
		ltick := time.NewTicker(time.Millisecond * 100)
		defer ltick.Stop()
		qlen := len(queue)

	loader:
		for {
			select {
			case <-ctx.Done():
				sl.sess.Log().Warn("loader context done")
				for _, status := range queue {
					if !status.Running() {
						sl.addErr(fmt.Errorf("service did not load on time %s", status.Addr().String()))
					}
				}
				sl.cancel(ctx.Err())
				return
			case <-ltick.C:
				var loaded int
				for _, status := range queue {
					if errs := status.Errs(); errs != nil {
						for _, err := range errs {
							sl.addErr(err)
						}
						var addr string
						if status.Addr() != nil {
							addr = status.Addr().String()
						}
						sl.cancel(fmt.Errorf("%w: service loader failed to load required services %s, %s", Error, addr, errors.Join(sl.errs...)))
						return
					}
					if status.Running() {
						loaded++
					}
				}
				if loaded == qlen {
					break loader
				}
			}
		}
		sl.done()
	}()

	return sl.loaderCh
}

func startEvent(svcs ...string) events.Event {
	payload := new(vars.Map)
	var errs []error
	for i, url := range svcs {
		if err := payload.Store(fmt.Sprintf("service.%d", i), url); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		_ = payload.Store("err", errors.Join(errs...).Error())
	}

	return StartEvent.Create(fmt.Sprintf("requested services (%d)", len(svcs)), payload)
}

func (sl *ServiceLoader) Err() error {
	if sl.loading {
		return fmt.Errorf("%w: service loader error checked before loader finished! did you wait for .Loaded?", Error)
	}
	return errors.Join(sl.errs...)
}

// cancel is used internally to cancel loading
func (sl *ServiceLoader) cancel(reason error) {
	sl.sess.Log().Warn("sevice loader canceled", slog.String("reason", reason.Error()))
	sl.addErr(reason)
	sl.loading = false
	close(sl.loaderCh)
}

func (sl *ServiceLoader) done() {
	sl.loading = false
	sl.sess.Log().Debug("service loader completed")
	close(sl.loaderCh)
}

func (sl *ServiceLoader) addErr(err error) {
	if err == nil {
		return
	}
	sl.errs = append(sl.errs, err)
}

type serviceCron struct {
	sess     *session.Context
	lib      *cron.Cron
	jobIDs   []cron.EntryID
	jobInfos map[cron.EntryID]cronInfo
}
type cronInfo struct {
	Name string
	Expr string
}

func newCron(sess *session.Context) *serviceCron {
	c := &serviceCron{
		jobInfos: make(map[cron.EntryID]cronInfo),
	}
	c.sess = sess
	c.lib = cron.New(cron.WithParser(cron.NewParser(
		cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)))
	return c
}

func (cs *serviceCron) Job(name, expr string, cb action.Action) {
	id, err := cs.lib.AddFunc(expr, func() {
		if err := cb(cs.sess); err != nil {
			cs.sess.Log().Error(fmt.Sprintf("%s:%s:%s", Error, cron.Error, err))
		}
	})
	cs.jobIDs = append(cs.jobIDs, id)
	cs.jobInfos[id] = cronInfo{name, expr}
	if err != nil {
		cs.sess.Log().Error(fmt.Sprintf(
			"%s:%s: failed to add job",
			Error,
			cron.Error), slog.Int("id", int(id)), slog.String("name", name), slog.String("expr", expr), slog.String("err", err.Error()))
		return
	}
}

func (cs *serviceCron) Start() error {
	if cs.sess.Get("app.services.cron_on_service_start").Bool() {
		for _, id := range cs.jobIDs {
			info, ok := cs.jobInfos[id]
			if !ok {
				cs.sess.Log().Error(fmt.Sprintf("%w:%w: failed to find job info", Error, cron.Error), slog.Int("id", int(id)))
				continue
			}
			internal.Log(cs.sess.Log(), "executing cron first time", slog.Int("job-id", int(id)), slog.String("name", info.Name), slog.String("expr", info.Expr))
			job := cs.lib.Entry(id)
			if job.Job != nil {
				go job.Job.Run()
			}
		}
	}
	cs.lib.Start()
	return nil
}

func (cs *serviceCron) Stop() error {
	ctx := cs.lib.Stop()
	<-ctx.Done()
	return nil
}
