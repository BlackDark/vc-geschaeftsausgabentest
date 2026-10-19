package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/app"
)

type passwordBody struct {
	Current string `json:"current"`
	Next    string `json:"next" minLength:"10" maxLength:"500"`
}

// LocationOption is a selectable rate location.
type LocationOption struct {
	Country string `json:"country"`
	City    string `json:"city"`
	Label   string `json:"label"`
}

type yearQuery struct {
	Year int `query:"year" minimum:"2000" maximum:"2100"`
}

// EmployerOut wraps the optional employer.
type EmployerOut struct {
	Employer *app.Employer `json:"employer,omitempty" doc:"absent until the employer is set up"`
}

type limitQuery struct {
	Limit int `query:"limit" minimum:"1" maximum:"500" default:"100"`
}

func (s *Server) registerProfile(api huma.API) {
	authed(s, api, huma.Operation{OperationID: "getMe", Method: http.MethodGet, Path: "/api/me", Tags: []string{"profile"}},
		func(ctx context.Context, u *app.User, _ *empty) (*body[*app.User], error) {
			return &body[*app.User]{Body: u}, nil
		})
	authed(s, api, huma.Operation{OperationID: "updateMe", Method: http.MethodPut, Path: "/api/me", Tags: []string{"profile"}},
		func(ctx context.Context, u *app.User, in *body[app.ProfileInput]) (*body[*app.User], error) {
			return wrap(s.app.UpdateProfile(ctx, u, in.Body))
		})
	authed(s, api, huma.Operation{OperationID: "changePassword", Method: http.MethodPost, Path: "/api/me/password", Tags: []string{"profile"}, DefaultStatus: 204},
		func(ctx context.Context, u *app.User, in *body[passwordBody]) (*empty, error) {
			return &empty{}, s.app.ChangePassword(ctx, u, in.Body.Current, in.Body.Next)
		})
	authed(s, api, huma.Operation{OperationID: "getEmployer", Method: http.MethodGet, Path: "/api/employer", Tags: []string{"profile"}},
		func(ctx context.Context, u *app.User, _ *empty) (*body[EmployerOut], error) {
			e, err := s.app.DefaultEmployer(ctx, u.ID)
			if err == app.ErrNotFound {
				return &body[EmployerOut]{}, nil
			}
			return wrap(EmployerOut{Employer: e}, err)
		})
	authed(s, api, huma.Operation{OperationID: "saveEmployer", Method: http.MethodPut, Path: "/api/employer", Tags: []string{"profile"}},
		func(ctx context.Context, u *app.User, in *body[app.EmployerInput]) (*body[*app.Employer], error) {
			return wrap(s.app.SaveEmployer(ctx, u, in.Body))
		})
	authed(s, api, huma.Operation{OperationID: "listLocations", Method: http.MethodGet, Path: "/api/locations", Tags: []string{"rates"}, Summary: "Rate locations for a year"},
		func(ctx context.Context, u *app.User, in *yearQuery) (*body[[]LocationOption], error) {
			rows, err := s.app.DB.QueryContext(ctx, `SELECT m.country, m.city, coalesce(c.bmf_name, m.label) FROM meal_rates m LEFT JOIN countries c ON c.iso = m.country WHERE m.year = ? AND m.active = 1 ORDER BY 3, m.city`, in.Year)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			out := []LocationOption{}
			for rows.Next() {
				var l LocationOption
				if err := rows.Scan(&l.Country, &l.City, &l.Label); err != nil {
					return nil, err
				}
				out = append(out, l)
			}
			return &body[[]LocationOption]{Body: out}, rows.Err()
		})
	authed(s, api, huma.Operation{OperationID: "listAudit", Method: http.MethodGet, Path: "/api/audit", Tags: []string{"profile"}, Summary: "Own audit trail"},
		func(ctx context.Context, u *app.User, in *limitQuery) (*body[[]app.AuditEntry], error) {
			return wrap(s.app.Audit(ctx, u.ID, in.Limit))
		})
}
