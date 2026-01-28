package app

import (
	"encoding/json"
	"net/http"

	"github.com/go-errors/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/metric"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/model"
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
)

// createFooHandler godoc
// @Summary		Creates foo
// @Description	Creates foo
// @Tags			createFoo
// @Accept			json
// @Produce		json
// @Success		200	{object}	model.CreateFooRequest
// @Failure		404	{object}	interface{}
// @Router			/api/v1/foo [post]
func (a *App) createFooHandler(w http.ResponseWriter, r *http.Request) (
	int, error) {

	var i model.CreateFooRequest
	if err := json.NewDecoder(r.Body).Decode(&i); err != nil {
		return http.StatusBadRequest, errors.New(err)
	}

	err := a.db.Create_Foo(&model.FooType{
		UUID:  i.UUID,
		Name:  i.Name,
		Email: i.Email,
	})
	if err != nil {
		return http.StatusInternalServerError, err
	}

	metric.FooCounter.With(prometheus.Labels{
		"tenant_id": "get_tenant_id_from_header",
		"foo_id":    i.UUID,
	}).Inc()
	return eh.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status": "created",
		"foo":    i,
	})
}

// getFooHandler godoc
// @Summary		Get foo
// @Description	Gets foo
// @Tags			getFoo
// @Param			id	path	string	true	"Foo ID"
// @Produce		json
// @Success		200	{object}	model.FooType
// @Failure		404	{object}	interface{}
// @Router			/api/v1/foo/{id} [get]
func (a *App) getFooHandler(w http.ResponseWriter, r *http.Request) (
	int, error) {

	id, err := eh.GetURLPathUUID(r, "id")
	if err != nil {
		return http.StatusBadRequest, err
	}

	foo, err := a.db.Get_Foo_By_UUID(id)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if foo == nil {
		return http.StatusNotFound, errors.Errorf("foo [%s] not found", id)
	}

	return eh.RespondWithJSON(w, http.StatusOK, foo)
}

// getCfnDummyHandler godoc
// @Summary		Get CFN dummy data
// @Description	Returns mock CFN data
// @Tags			cfn
// @Produce		json
// @Success		200	{object}	interface{}
// @Router			/api/v1/cfn/dummy [get]
func (a *App) getCfnDummyHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	return eh.RespondWithJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "cfn dummy response",
	})
}
