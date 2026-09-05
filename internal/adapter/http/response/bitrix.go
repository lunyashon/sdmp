package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type BitrixTime struct {
	Start      float64 `json:"start"`
	Finish     float64 `json:"finish"`
	Duration   float64 `json:"duration"`
	Processing float64 `json:"processing"`
	DateStart  string  `json:"date_start"`
	DateFinish string  `json:"date_finish"`
}

type BitrixOK struct {
	Result int64      `json:"result"`
	Time   BitrixTime `json:"time"`
}

type BitrixErrorBody struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func BitrixResult(c *gin.Context, start time.Time, id int64) {
	finish := time.Now()
	duration := finish.Sub(start).Seconds()
	c.JSON(http.StatusOK, BitrixOK{
		Result: id,
		Time: BitrixTime{
			Start:      unixFloat(start),
			Finish:     unixFloat(finish),
			Duration:   duration,
			Processing: duration,
			DateStart:  start.Format(time.RFC3339),
			DateFinish: finish.Format(time.RFC3339),
		},
	})
}

func BitrixError(c *gin.Context, status int, code, description string) {
	c.JSON(status, BitrixErrorBody{
		Error:            code,
		ErrorDescription: description,
	})
}

func unixFloat(t time.Time) float64 {
	return float64(t.UnixNano()) / 1e9
}
