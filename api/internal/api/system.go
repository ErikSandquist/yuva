package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/productdevbook/yuva/api/internal/oas"
)

func (s *Server) GetHealthz(context.Context, oas.GetHealthzRequestObject) (oas.GetHealthzResponseObject, error) {
	return oas.GetHealthz200JSONResponse{Status: oas.Ok}, nil
}

func (s *Server) GetReadyz(ctx context.Context, _ oas.GetReadyzRequestObject) (oas.GetReadyzResponseObject, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.st.Pool.Ping(ctx); err != nil {
		s.log.WarnContext(ctx, "not ready", slog.Any("error", err))
		return oas.GetReadyz503ApplicationProblemPlusJSONResponse(errNotReady.body()), nil
	}
	return oas.GetReadyz200JSONResponse{Status: oas.Ok}, nil
}

func (s *Server) GetVersion(context.Context, oas.GetVersionRequestObject) (oas.GetVersionResponseObject, error) {
	signup := oas.VersionSignupInvite
	if s.auth.Signup.Open {
		signup = oas.VersionSignupOpen
	}
	out := oas.GetVersion200JSONResponse{Version: s.version, Signup: signup}
	if s.auth.LocaleCookieDomain != "" {
		out.LocaleCookieDomain = &s.auth.LocaleCookieDomain
	}
	return out, nil
}
