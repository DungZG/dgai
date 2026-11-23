package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type priceScheduleRequest struct {
	ModelName   string  `json:"model_name"`
	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
	CachePrice  float64 `json:"cache_price"`
	SaleTag     string  `json:"sale_tag"`
	EffectiveAt int64   `json:"effective_at"`
	Announce    *bool   `json:"announce"`
	Note        string  `json:"note"`
}

// CreatePriceSchedule arms a price change; a change whose time has already come
// is applied immediately through the same scheduler path.
func CreatePriceSchedule(c *gin.Context) {
	var request priceScheduleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	request.ModelName = strings.TrimSpace(request.ModelName)
	if request.ModelName == "" {
		common.ApiErrorMsg(c, "model_name is required")
		return
	}
	if request.InputPrice <= 0 || request.OutputPrice <= 0 {
		common.ApiErrorMsg(c, "input_price and output_price must be positive")
		return
	}
	if request.CachePrice <= 0 {
		request.CachePrice = request.InputPrice * 0.3
	}
	announce := true
	if request.Announce != nil {
		announce = *request.Announce
	}
	effectiveAt := request.EffectiveAt
	if effectiveAt <= 0 {
		effectiveAt = common.GetTimestamp()
	}
	row := &model.PriceSchedule{
		ModelName:     request.ModelName,
		NewRatio:      request.InputPrice / 2,
		NewCompletion: request.OutputPrice / request.InputPrice,
		NewCache:      request.CachePrice / request.InputPrice,
		SaleTag:       strings.TrimSpace(request.SaleTag),
		EffectiveAt:   effectiveAt,
		Status:        model.PriceSchedulePending,
		Announce:      announce,
		Note:          request.Note,
		CreatedBy:     c.GetInt("id"),
	}
	if err := model.CreatePriceSchedule(row); err != nil {
		common.ApiError(c, err)
		return
	}
	if effectiveAt <= common.GetTimestamp() {
		if err := service.ApplyPriceSchedule(row.Id); err != nil {
			common.ApiError(c, err)
			return
		}
		if stored, err := model.GetPriceSchedule(row.Id); err == nil {
			row = stored
		}
	}
	common.ApiSuccess(c, row)
}

func ListPriceSchedules(c *gin.Context) {
	var rows []model.PriceSchedule
	if err := model.DB.Order("id desc").Limit(50).Find(&rows).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func CancelPriceSchedule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid id")
		return
	}
	cancelled, err := model.CancelPriceSchedule(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !cancelled {
		common.ApiErrorMsg(c, "price change is not pending")
		return
	}
	common.ApiSuccess(c, nil)
}
