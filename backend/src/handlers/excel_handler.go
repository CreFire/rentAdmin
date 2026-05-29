package handlers

import (
	"database/sql"
	"net/http"

	"rentadmin/src/services"

	"github.com/gin-gonic/gin"
)

// ExcelHandler handles Excel file import requests.
type ExcelHandler struct {
	DB *sql.DB
}

// ImportExcel handles POST /api/excel/import.
func (h *ExcelHandler) ImportExcel(c *gin.Context) {
	summary, err := services.ImportTenantsFromExcel(h.DB, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}
