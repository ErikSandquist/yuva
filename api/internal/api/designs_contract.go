package api

import (
	"context"
	"net/http"

	"github.com/productdevbook/yuva/api/internal/oas"
)

var errDesignsNotYet = problem(http.StatusNotImplemented, "not_implemented", "e-mail designs are not served yet")

func (s *Server) ListEmailTemplates(context.Context, oas.ListEmailTemplatesRequestObject) (oas.ListEmailTemplatesResponseObject, error) {
	return nil, errDesignsNotYet
}

func (s *Server) GetEmailTemplate(context.Context, oas.GetEmailTemplateRequestObject) (oas.GetEmailTemplateResponseObject, error) {
	return nil, errDesignsNotYet
}

func (s *Server) ListEmailDesigns(context.Context, oas.ListEmailDesignsRequestObject) (oas.ListEmailDesignsResponseObject, error) {
	return nil, errDesignsNotYet
}

func (s *Server) CreateEmailDesign(context.Context, oas.CreateEmailDesignRequestObject) (oas.CreateEmailDesignResponseObject, error) {
	return nil, errDesignsNotYet
}

func (s *Server) RenderEmailDesign(context.Context, oas.RenderEmailDesignRequestObject) (oas.RenderEmailDesignResponseObject, error) {
	return nil, errDesignsNotYet
}

func (s *Server) GetEmailDesign(context.Context, oas.GetEmailDesignRequestObject) (oas.GetEmailDesignResponseObject, error) {
	return nil, errDesignsNotYet
}

func (s *Server) UpdateEmailDesign(context.Context, oas.UpdateEmailDesignRequestObject) (oas.UpdateEmailDesignResponseObject, error) {
	return nil, errDesignsNotYet
}

func (s *Server) ArchiveEmailDesign(context.Context, oas.ArchiveEmailDesignRequestObject) (oas.ArchiveEmailDesignResponseObject, error) {
	return nil, errDesignsNotYet
}

func (s *Server) DuplicateEmailDesign(context.Context, oas.DuplicateEmailDesignRequestObject) (oas.DuplicateEmailDesignResponseObject, error) {
	return nil, errDesignsNotYet
}

func (s *Server) ListEmailDesignVersions(context.Context, oas.ListEmailDesignVersionsRequestObject) (oas.ListEmailDesignVersionsResponseObject, error) {
	return nil, errDesignsNotYet
}

func (s *Server) GetEmailDesignVersion(context.Context, oas.GetEmailDesignVersionRequestObject) (oas.GetEmailDesignVersionResponseObject, error) {
	return nil, errDesignsNotYet
}

func (s *Server) RestoreEmailDesignVersion(context.Context, oas.RestoreEmailDesignVersionRequestObject) (oas.RestoreEmailDesignVersionResponseObject, error) {
	return nil, errDesignsNotYet
}

func (s *Server) ListEmailImages(context.Context, oas.ListEmailImagesRequestObject) (oas.ListEmailImagesResponseObject, error) {
	return nil, errDesignsNotYet
}

func (s *Server) UploadEmailImage(context.Context, oas.UploadEmailImageRequestObject) (oas.UploadEmailImageResponseObject, error) {
	return nil, errDesignsNotYet
}

func (s *Server) DeleteEmailImage(context.Context, oas.DeleteEmailImageRequestObject) (oas.DeleteEmailImageResponseObject, error) {
	return nil, errDesignsNotYet
}
