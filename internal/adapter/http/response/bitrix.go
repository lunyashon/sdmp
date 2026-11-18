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
	BitrixData(c, start, id)
}

func BitrixData(c *gin.Context, start time.Time, result any) {
	c.JSON(http.StatusOK, gin.H{
		"result": result,
		"time":   bitrixTime(start),
	})
}

func BitrixList(c *gin.Context, start time.Time, result any, total int) {
	c.JSON(http.StatusOK, gin.H{
		"result": result,
		"total":  total,
		"time":   bitrixTime(start),
	})
}

func bitrixTime(start time.Time) BitrixTime {
	finish := time.Now()
	duration := finish.Sub(start).Seconds()
	return BitrixTime{
		Start:      unixFloat(start),
		Finish:     unixFloat(finish),
		Duration:   duration,
		Processing: duration,
		DateStart:  start.Format(time.RFC3339),
		DateFinish: finish.Format(time.RFC3339),
	}
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
