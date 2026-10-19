package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark/vc-geschaeftsausgabentest/internal/app"
)

func (s *Server) registerTrips(api huma.API) {
	tags := []string{"trips"}
	authed(s, api, huma.Operation{OperationID: "getDashboard", Method: http.MethodGet, Path: "/api/dashboard", Tags: []string{"overview"}},
		func(ctx context.Context, u *app.User, _ *empty) (*body[*app.Dashboard], error) {
			return wrap(s.app.Dashboard(ctx, u))
		})
	authed(s, api, huma.Operation{OperationID: "listTrips", Method: http.MethodGet, Path: "/api/trips", Tags: tags},
		func(ctx context.Context, u *app.User, _ *empty) (*body[[]app.TripListItem], error) {
			return wrap(s.app.TripList(ctx, u))
		})
	authed(s, api, huma.Operation{OperationID: "createTrip", Method: http.MethodPost, Path: "/api/trips", Tags: tags, DefaultStatus: 201},
		func(ctx context.Context, u *app.User, in *body[app.TripInput]) (*body[*app.TripDetail], error) {
			t, err := s.app.CreateTrip(ctx, u, in.Body)
			if err != nil {
				return nil, err
			}
			return wrap(s.app.TripDetail(ctx, u, t.ID))
		})
	authed(s, api, huma.Operation{OperationID: "getTrip", Method: http.MethodGet, Path: "/api/trips/{id}", Tags: tags},
		func(ctx context.Context, u *app.User, in *idIn) (*body[*app.TripDetail], error) {
			return wrap(s.app.TripDetail(ctx, u, in.ID))
		})
	authed(s, api, huma.Operation{OperationID: "updateTrip", Method: http.MethodPut, Path: "/api/trips/{id}", Tags: tags},
		func(ctx context.Context, u *app.User, in *idBody[app.TripInput]) (*body[*app.TripDetail], error) {
			if _, err := s.app.UpdateTrip(ctx, u, in.ID, in.Body); err != nil {
				return nil, err
			}
			return wrap(s.app.TripDetail(ctx, u, in.ID))
		})
	authed(s, api, huma.Operation{OperationID: "deleteTrip", Method: http.MethodDelete, Path: "/api/trips/{id}", Tags: tags, DefaultStatus: 204},
		func(ctx context.Context, u *app.User, in *idIn) (*empty, error) {
			return &empty{}, s.app.DeleteTrip(ctx, u, in.ID)
		})
}

func (s *Server) registerExpenses(api huma.API) {
	tags := []string{"expenses"}
	authed(s, api, huma.Operation{OperationID: "listOutlays", Method: http.MethodGet, Path: "/api/outlays", Tags: tags, Summary: "Standalone outlays"},
		func(ctx context.Context, u *app.User, _ *empty) (*body[[]app.OutlayListItem], error) {
			return wrap(s.app.OutlayList(ctx, u))
		})
	authed(s, api, huma.Operation{OperationID: "createExpense", Method: http.MethodPost, Path: "/api/expenses", Tags: tags, DefaultStatus: 201},
		func(ctx context.Context, u *app.User, in *body[app.ExpenseInput]) (*body[*app.Expense], error) {
			return wrap(s.app.CreateExpense(ctx, u, in.Body))
		})
	authed(s, api, huma.Operation{OperationID: "getExpense", Method: http.MethodGet, Path: "/api/expenses/{id}", Tags: tags},
		func(ctx context.Context, u *app.User, in *idIn) (*body[*app.Expense], error) {
			return wrap(s.app.GetExpense(ctx, u, in.ID))
		})
	authed(s, api, huma.Operation{OperationID: "updateExpense", Method: http.MethodPut, Path: "/api/expenses/{id}", Tags: tags},
		func(ctx context.Context, u *app.User, in *idBody[app.ExpenseInput]) (*body[*app.Expense], error) {
			return wrap(s.app.UpdateExpense(ctx, u, in.ID, in.Body))
		})
	authed(s, api, huma.Operation{OperationID: "deleteExpense", Method: http.MethodDelete, Path: "/api/expenses/{id}", Tags: tags, DefaultStatus: 204},
		func(ctx context.Context, u *app.User, in *idIn) (*empty, error) {
			return &empty{}, s.app.DeleteExpense(ctx, u, in.ID)
		})
	authed(s, api, huma.Operation{OperationID: "listAdvances", Method: http.MethodGet, Path: "/api/advances", Tags: tags},
		func(ctx context.Context, u *app.User, _ *empty) (*body[[]*app.Advance], error) {
			return wrap(s.app.ListAdvances(ctx, u))
		})
	authed(s, api, huma.Operation{OperationID: "createAdvance", Method: http.MethodPost, Path: "/api/advances", Tags: tags, DefaultStatus: 201},
		func(ctx context.Context, u *app.User, in *body[app.AdvanceInput]) (*body[*app.Advance], error) {
			return wrap(s.app.CreateAdvance(ctx, u, in.Body))
		})
	authed(s, api, huma.Operation{OperationID: "deleteAdvance", Method: http.MethodDelete, Path: "/api/advances/{id}", Tags: tags, DefaultStatus: 204},
		func(ctx context.Context, u *app.User, in *idIn) (*empty, error) {
			return &empty{}, s.app.DeleteAdvance(ctx, u, in.ID)
		})
}

type receiptListQuery struct {
	Status string `query:"status" enum:"inbox,confirmed,void,"`
}

type uploadIn struct {
	Filename string `query:"filename" maxLength:"255"`
	Note     string `query:"note" maxLength:"500"`
	RawBody  []byte `contentType:"application/octet-stream"`
}

type fileOut struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	CacheControl       string `header:"Cache-Control"`
	Body               []byte
}

func attachment(name string, inline bool) string {
	kind := "attachment"
	if inline {
		kind = "inline"
	}
	return fmt.Sprintf("%s; filename*=UTF-8''%s", kind, url.PathEscape(name))
}

func (s *Server) registerReceipts(api huma.API) {
	tags := []string{"receipts"}
	authed(s, api, huma.Operation{OperationID: "listReceipts", Method: http.MethodGet, Path: "/api/receipts", Tags: tags},
		func(ctx context.Context, u *app.User, in *receiptListQuery) (*body[[]*app.Receipt], error) {
			list, err := s.app.ListReceipts(ctx, u, in.Status)
			return wrap(nonNilSlice(list), err)
		})
	authed(s, api, huma.Operation{OperationID: "uploadReceipt", Method: http.MethodPost, Path: "/api/receipts", Tags: tags, DefaultStatus: 201, MaxBodyBytes: 26 << 20,
		Summary: "Upload an original receipt (JPEG, PNG or PDF) into the inbox"},
		func(ctx context.Context, u *app.User, in *uploadIn) (*body[*app.Receipt], error) {
			return wrap(s.app.Upload(ctx, u, in.Filename, in.RawBody, in.Note))
		})
	authed(s, api, huma.Operation{OperationID: "createSelfIssued", Method: http.MethodPost, Path: "/api/receipts/self-issued", Tags: tags, DefaultStatus: 201, Summary: "Create an Eigenbeleg"},
		func(ctx context.Context, u *app.User, in *body[app.SelfIssued]) (*body[*app.Receipt], error) {
			return wrap(s.app.CreateSelfIssued(ctx, u, in.Body))
		})
	authed(s, api, huma.Operation{OperationID: "getReceipt", Method: http.MethodGet, Path: "/api/receipts/{id}", Tags: tags},
		func(ctx context.Context, u *app.User, in *idIn) (*body[*app.Receipt], error) {
			return wrap(s.app.GetReceipt(ctx, u, in.ID))
		})
	authed(s, api, huma.Operation{OperationID: "confirmReceipt", Method: http.MethodPost, Path: "/api/receipts/{id}/confirm", Tags: tags, Summary: "Confirm after visual check; assigns the receipt number"},
		func(ctx context.Context, u *app.User, in *idIn) (*body[*app.Receipt], error) {
			return wrap(s.app.ConfirmReceipt(ctx, u, in.ID))
		})
	authed(s, api, huma.Operation{OperationID: "voidReceipt", Method: http.MethodPost, Path: "/api/receipts/{id}/void", Tags: tags},
		func(ctx context.Context, u *app.User, in *idBody[reasonBody]) (*body[*app.Receipt], error) {
			return wrap(s.app.VoidReceipt(ctx, u, in.ID, in.Body.Reason))
		})
	authed(s, api, huma.Operation{OperationID: "deleteReceipt", Method: http.MethodDelete, Path: "/api/receipts/{id}", Tags: tags, DefaultStatus: 204, Summary: "Delete an unconfirmed upload"},
		func(ctx context.Context, u *app.User, in *idIn) (*empty, error) {
			return &empty{}, s.app.DeleteInboxReceipt(ctx, u, in.ID)
		})
	file := func(preview bool) func(ctx context.Context, u *app.User, in *idIn) (*fileOut, error) {
		return func(ctx context.Context, u *app.User, in *idIn) (*fileOut, error) {
			b, mime, name, err := s.app.ReceiptFile(ctx, u, in.ID, preview)
			if err != nil {
				return nil, err
			}
			return &fileOut{ContentType: mime, ContentDisposition: attachment(name, true), CacheControl: "private, max-age=3600", Body: b}, nil
		}
	}
	authed(s, api, huma.Operation{OperationID: "getReceiptFile", Method: http.MethodGet, Path: "/api/receipts/{id}/file", Tags: tags, Summary: "Original bytes"}, file(false))
	authed(s, api, huma.Operation{OperationID: "getReceiptPreview", Method: http.MethodGet, Path: "/api/receipts/{id}/preview", Tags: tags, Summary: "PNG preview of the first page"}, file(true))
}

type claimItemBody struct {
	app.ClaimItem
	Include bool `json:"include"`
}

type versionIn struct {
	ID      int64 `path:"id"`
	Version int   `path:"version"`
}

func (s *Server) registerClaims(api huma.API) {
	tags := []string{"claims"}
	authed(s, api, huma.Operation{OperationID: "listClaims", Method: http.MethodGet, Path: "/api/claims", Tags: tags},
		func(ctx context.Context, u *app.User, _ *empty) (*body[[]*app.Claim], error) {
			list, err := s.app.ListClaims(ctx, u)
			return wrap(nonNilSlice(list), err)
		})
	authed(s, api, huma.Operation{OperationID: "createClaim", Method: http.MethodPost, Path: "/api/claims", Tags: tags, DefaultStatus: 201},
		func(ctx context.Context, u *app.User, in *body[app.ClaimInput]) (*body[*app.Claim], error) {
			return wrap(s.app.CreateClaim(ctx, u, in.Body))
		})
	authed(s, api, huma.Operation{OperationID: "getClaim", Method: http.MethodGet, Path: "/api/claims/{id}", Tags: tags, Summary: "Claim with preview, blockers, warnings and suggestions"},
		func(ctx context.Context, u *app.User, in *idIn) (*body[*app.Preview], error) {
			return wrap(s.app.PreviewClaim(ctx, u, in.ID))
		})
	authed(s, api, huma.Operation{OperationID: "updateClaim", Method: http.MethodPut, Path: "/api/claims/{id}", Tags: tags},
		func(ctx context.Context, u *app.User, in *idBody[app.ClaimInput]) (*body[*app.Claim], error) {
			return wrap(s.app.UpdateClaim(ctx, u, in.ID, in.Body))
		})
	authed(s, api, huma.Operation{OperationID: "deleteClaim", Method: http.MethodDelete, Path: "/api/claims/{id}", Tags: tags, DefaultStatus: 204},
		func(ctx context.Context, u *app.User, in *idIn) (*empty, error) {
			return &empty{}, s.app.DeleteClaim(ctx, u, in.ID)
		})
	authed(s, api, huma.Operation{OperationID: "setClaimItem", Method: http.MethodPost, Path: "/api/claims/{id}/items", Tags: tags, Summary: "Add or remove a trip, outlay or advance"},
		func(ctx context.Context, u *app.User, in *idBody[claimItemBody]) (*body[*app.Claim], error) {
			return wrap(s.app.SetClaimItem(ctx, u, in.ID, in.Body.ClaimItem, in.Body.Include))
		})
	authed(s, api, huma.Operation{OperationID: "submitClaim", Method: http.MethodPost, Path: "/api/claims/{id}/submit", Tags: tags},
		func(ctx context.Context, u *app.User, in *idBody[app.SubmitInput]) (*body[*app.Claim], error) {
			return wrap(s.app.SubmitClaim(ctx, u, in.ID, in.Body))
		})
	authed(s, api, huma.Operation{OperationID: "unlockClaim", Method: http.MethodPost, Path: "/api/claims/{id}/unlock", Tags: tags},
		func(ctx context.Context, u *app.User, in *idBody[reasonBody]) (*body[*app.Claim], error) {
			return wrap(s.app.UnlockClaim(ctx, u, in.ID, in.Body.Reason))
		})
	authed(s, api, huma.Operation{OperationID: "markClaimPaid", Method: http.MethodPost, Path: "/api/claims/{id}/paid", Tags: tags},
		func(ctx context.Context, u *app.User, in *idBody[app.PaymentInput]) (*body[*app.Claim], error) {
			return wrap(s.app.MarkPaid(ctx, u, in.ID, in.Body))
		})
	authed(s, api, huma.Operation{OperationID: "undoClaimPaid", Method: http.MethodPost, Path: "/api/claims/{id}/unpaid", Tags: tags},
		func(ctx context.Context, u *app.User, in *idBody[reasonBody]) (*body[*app.Claim], error) {
			return wrap(s.app.UndoPaid(ctx, u, in.ID, in.Body.Reason))
		})
	download := func(kind string) func(ctx context.Context, u *app.User, in *versionIn) (*fileOut, error) {
		return func(ctx context.Context, u *app.User, in *versionIn) (*fileOut, error) {
			b, name, err := s.app.VersionFile(ctx, u, in.ID, in.Version, kind)
			if err != nil {
				return nil, err
			}
			ct := "application/pdf"
			if kind == "zip" {
				ct = "application/zip"
			}
			return &fileOut{ContentType: ct, ContentDisposition: attachment(name, kind == "pdf"), CacheControl: "private, no-store", Body: b}, nil
		}
	}
	authed(s, api, huma.Operation{OperationID: "downloadClaimPdf", Method: http.MethodGet, Path: "/api/claims/{id}/versions/{version}/pdf", Tags: tags}, download("pdf"))
	authed(s, api, huma.Operation{OperationID: "downloadClaimZip", Method: http.MethodGet, Path: "/api/claims/{id}/versions/{version}/zip", Tags: tags}, download("zip"))
}

type createUserBody struct {
	Username string `json:"username" minLength:"1" maxLength:"100"`
	Name     string `json:"name" minLength:"1" maxLength:"200"`
	Password string `json:"password" minLength:"10" maxLength:"500"`
	Admin    bool   `json:"admin" required:"false"`
}

type activeBody struct {
	Active bool   `json:"active"`
	Reason string `json:"reason,omitempty" maxLength:"500"`
}

func (s *Server) registerAdmin(api huma.API) {
	tags := []string{"admin"}
	authed(s, api, huma.Operation{OperationID: "listUsers", Method: http.MethodGet, Path: "/api/admin/users", Tags: tags},
		func(ctx context.Context, u *app.User, _ *empty) (*body[[]app.AccountSummary], error) {
			return wrap(s.app.ListUsers(ctx, u))
		})
	authed(s, api, huma.Operation{OperationID: "createUser", Method: http.MethodPost, Path: "/api/admin/users", Tags: tags, DefaultStatus: 201},
		func(ctx context.Context, u *app.User, in *body[createUserBody]) (*body[*app.User], error) {
			return wrap(s.app.CreateUser(ctx, u, in.Body.Username, in.Body.Name, in.Body.Password, in.Body.Admin))
		})
	authed(s, api, huma.Operation{OperationID: "setUserActive", Method: http.MethodPost, Path: "/api/admin/users/{id}/active", Tags: tags, DefaultStatus: 204},
		func(ctx context.Context, u *app.User, in *idBody[activeBody]) (*empty, error) {
			return &empty{}, s.app.SetActive(ctx, u, in.ID, in.Body.Active, in.Body.Reason)
		})
}

func nonNilSlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
