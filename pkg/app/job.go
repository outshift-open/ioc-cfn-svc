package app

import (
	"time"
)

func (a *App) LongRunningBackgroundJob() {
	log := getLogger()

	defer func() {
		if panicErr := recover(); panicErr != nil {
			log.Errorf("recovered from panic: [%s]", panicErr)
		}
	}()

	log.Info("long running job triggered")
	ticker := time.NewTicker(time.Hour * 2)
	for {
		select {
		case <-a.stopChan:
			log.Infof("app stopped. ejecting from job")
			return
		case <-ticker.C:
			err := a.runLongJob()
			if err != nil {
				log.Warnf("error running job: %s", err)
			}
		}
	}
}

func (a *App) runLongJob() error {
	log := getLogger()

	// do something
	log.Info("running job")
	return nil
}
