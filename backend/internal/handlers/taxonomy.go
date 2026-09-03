package handlers

import (
	"net/http"
	"strconv"

	"erro-notebook/backend/internal/repository"
	"erro-notebook/backend/internal/services"
	"erro-notebook/backend/pkg/response"
	"github.com/gin-gonic/gin"
)

type TaxonomyHandler struct {
	taxonomyService *services.TaxonomyService
	questionRepo    *repository.QuestionRepository
}

func NewTaxonomyHandler(taxonomyService *services.TaxonomyService, questionRepo *repository.QuestionRepository) *TaxonomyHandler {
	return &TaxonomyHandler{taxonomyService: taxonomyService, questionRepo: questionRepo}
}

func (h *TaxonomyHandler) ListCategories(c *gin.Context) {
	items, err := h.taxonomyService.ListCategories()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "CATEGORY_LIST_FAILED", err.Error())
		return
	}
	response.OK(c, items)
}

type createCategoryRequest struct {
	Name     string `json:"name" binding:"required"`
	ParentID *int64 `json:"parentId"`
}

func (h *TaxonomyHandler) CreateCategory(c *gin.Context) {
	var req createCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	item, err := h.taxonomyService.CreateCategory(req.Name, req.ParentID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "CATEGORY_CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, item)
}

func (h *TaxonomyHandler) ListTags(c *gin.Context) {
	items, err := h.taxonomyService.ListTags()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "TAG_LIST_FAILED", err.Error())
		return
	}
	response.OK(c, items)
}

type createTagRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *TaxonomyHandler) CreateTag(c *gin.Context) {
	var req createTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	item, err := h.taxonomyService.CreateTag(req.Name)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "TAG_CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, item)
}

func (h *TaxonomyHandler) UpdateCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid id")
		return
	}

	var req createCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	category, err := h.taxonomyService.UpdateCategory(id, req.Name, req.ParentID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "CATEGORY_UPDATE_FAILED", err.Error())
		return
	}
	if category == nil {
		response.Error(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", "category not found")
		return
	}

	response.OK(c, category)
}

func (h *TaxonomyHandler) DeleteCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid id")
		return
	}

	if err := h.taxonomyService.DeleteCategory(id); err != nil {
		response.Error(c, http.StatusInternalServerError, "CATEGORY_DELETE_FAILED", err.Error())
		return
	}

	response.NoContent(c)
}

func (h *TaxonomyHandler) DeleteTag(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid id")
		return
	}

	if err := h.taxonomyService.DeleteTag(id); err != nil {
		response.Error(c, http.StatusInternalServerError, "TAG_DELETE_FAILED", err.Error())
		return
	}

	response.NoContent(c)
}

func (h *TaxonomyHandler) GetCategoryTree(c *gin.Context) {
	counts, err := h.questionRepo.CountByCategory()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "CATEGORY_TREE_FAILED", err.Error())
		return
	}
	uncategorized, err := h.questionRepo.CountUncategorized()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "CATEGORY_TREE_FAILED", err.Error())
		return
	}
	tree, err := h.taxonomyService.GetCategoryTree(counts)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "CATEGORY_TREE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{
		"categories":    tree,
		"uncategorized": uncategorized,
	})
}
