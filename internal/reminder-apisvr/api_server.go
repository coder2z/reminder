/**
* @Author: myxy99 <myxy99@foxmail.com>
* @Date: 2020/11/4 11:14
 */
package reminder_apisvr

import (
	"context"
	fmt "fmt"
	v1 "github.com/myxy99/reminder/internal/reminder-apisvr/api/v1"
	"github.com/myxy99/reminder/internal/reminder-apisvr/config"
	"github.com/myxy99/reminder/internal/reminder-apisvr/models"
	"github.com/myxy99/reminder/internal/reminder-apisvr/server"
	myValidator "github.com/myxy99/reminder/internal/reminder-apisvr/validator"
	"github.com/myxy99/reminder/pkg/client/database"
	"github.com/myxy99/reminder/pkg/client/rabbitmq"
	"github.com/myxy99/reminder/pkg/log"
	"github.com/myxy99/reminder/pkg/reminder"
	"github.com/myxy99/reminder/pkg/validator"
	"net/http"
)

type WebServer struct {
	DB *database.Client

	Config *config.Cfg

	Server *http.Server

	Validator *validator.Validator

	CronServer *reminder.CronServer

	Mq *rabbitmq.RabbitMQ
}

func (s *WebServer) PrepareRun(stopCh <-chan struct{}) (err error) {
	err = s.installCfg()
	if err != nil {
		return
	}

	err = s.installLog()
	if err != nil {
		return
	}

	err = s.installDatabase(stopCh)
	if err != nil {
		return
	}

	s.installWX()

	s.installHttpServer()

	err = s.installValidator()
	if err != nil {
		return
	}

	s.installAPIs()

	err = s.installRabbitMQ(stopCh)
	if err != nil {
		return
	}

	s.installReminder(stopCh)

	return nil
}

func (s *WebServer) Run(stopCh <-chan struct{}) (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		_ = s.Server.Shutdown(ctx)
	}()
	log.Info(fmt.Sprintf("Start listening on %s", s.Server.Addr))
	err = s.Server.ListenAndServe()
	return err
}

func (s *WebServer) migration() {
	s.DB.DB().AutoMigrate(
		new(models.User),
		new(models.Time),
		new(models.Remind),
	)
}

func (s *WebServer) installWX() {
	server.NewWxAPP(s.Config.WXApp.Secret, s.Config.WXApp.AppID)
}

func (s *WebServer) installReminder(stopCh <-chan struct{}) {
	s.CronServer = reminder.NewReminderClient(s.Config.Reminder)
	go s.CronServer.Run(stopCh, server.NewReminder(s.DB, s.Mq))
}

func (s *WebServer) installRabbitMQ(stopCh <-chan struct{}) (err error) {
	s.Mq, err = rabbitmq.NewRabbitMQSimple("reminder", s.Config.RabbitMq)
	go func() {
		<-stopCh
		s.Mq.Destory()
	}()
	return
}

func (s *WebServer) installAPIs() {
	s.Server.Handler = v1.InitRouter(s.DB, s.Validator)
}

func (s *WebServer) installLog() error {
	return log.NewLog(s.Config.Log)
}

func (s *WebServer) installHttpServer() {
	s.Server.Addr = s.Config.Server.Addr
}

func (s *WebServer) installValidator() error {
	s.Validator = validator.New()
	return s.Validator.InitTrans(s.Config.Server.Locale, myValidator.RegisterValidation)
}

func (s *WebServer) installDatabase(stopCh <-chan struct{}) (err error) {
	s.DB, err = database.NewDatabaseClient(s.Config.Database, stopCh)
	if err != nil {
		return
	}
	s.migration()
	return
}

func (s *WebServer) installCfg() (err error) {
	s.Config, err = config.TryLoadFromDisk()
	return
}
