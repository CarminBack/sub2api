package admin

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type rechargeMultiplierSettingRepo struct {
	values  map[string]string
	updates map[string]string
}

func (r *rechargeMultiplierSettingRepo) Get(context.Context, string) (*service.Setting, error) {
	return nil, nil
}
func (r *rechargeMultiplierSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], nil
}
func (r *rechargeMultiplierSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}
func (r *rechargeMultiplierSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = r.values[key]
	}
	return values, nil
}
func (r *rechargeMultiplierSettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	r.updates = make(map[string]string, len(values))
	for key, value := range values {
		r.values[key] = value
		r.updates[key] = value
	}
	return nil
}
func (r *rechargeMultiplierSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}
func (r *rechargeMultiplierSettingRepo) Delete(context.Context, string) error { return nil }

func TestRechargeMultiplierHandlerOnlyUpdatesDelegatedField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &rechargeMultiplierSettingRepo{values: map[string]string{
		service.SettingBalanceRechargeMult: "1.50",
		service.SettingBalanceRechargeMin:  "1.20",
		service.SettingPaymentEnabled:      "true",
	}}
	handler := NewPaymentHandler(nil, service.NewPaymentConfigService(nil, repo, nil))
	router := gin.New()
	router.PUT("/admin/payment/recharge-multiplier", handler.UpdateRechargeMultiplier)

	rec := doJSON(t, router, http.MethodPut, "/admin/payment/recharge-multiplier", map[string]any{
		"balance_recharge_multiplier": 1.25,
		"enabled":                     false,
		"recharge_fee_rate":           99,
	})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, map[string]string{service.SettingBalanceRechargeMult: "1.25"}, repo.updates)
	require.Equal(t, "true", repo.values[service.SettingPaymentEnabled])
}

func TestRechargeMultiplierHandlerRejectsBelowMinimum(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &rechargeMultiplierSettingRepo{values: map[string]string{
		service.SettingBalanceRechargeMult: "1.50",
		service.SettingBalanceRechargeMin:  "1.20",
	}}
	handler := NewPaymentHandler(nil, service.NewPaymentConfigService(nil, repo, nil))
	router := gin.New()
	router.PUT("/admin/payment/recharge-multiplier", handler.UpdateRechargeMultiplier)

	rec := doJSON(t, router, http.MethodPut, "/admin/payment/recharge-multiplier", map[string]any{
		"balance_recharge_multiplier": 1.19,
	})

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, repo.updates)
}
