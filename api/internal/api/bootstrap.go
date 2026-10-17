package api

import (
	"context"
	"errors"
	"strings"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

type BootstrapInput struct {
	Email         string
	Workspace     string
	Name          string
	Locale        string
	AllowExisting bool
}

type BootstrapResult struct {
	WorkspaceID uuid.UUID
	MemberID    uuid.UUID
	PersonID    uuid.UUID
}

func Bootstrap(ctx context.Context, st *store.Store, in BootstrapInput) (BootstrapResult, error) {
	email, err := normalizeEmail(oas.Email(in.Email))
	if err != nil {
		return BootstrapResult{}, errors.New("--email is not a valid address")
	}
	name, ok := workspaceName(in.Workspace)
	if !ok {
		return BootstrapResult{}, errors.New("--workspace must be 1 to 200 characters")
	}
	if in.Locale == "" {
		in.Locale = string(oas.En)
	}
	if !validLocale(oas.Locale(in.Locale)) {
		return BootstrapResult{}, errors.New("--locale must be a supported language code such as en, tr or pt-BR")
	}
	var res BootstrapResult
	err = st.InTx(ctx, func(q *store.Queries) error {
		if err := q.LockBootstrap(ctx); err != nil {
			return err
		}
		n, err := q.CountWorkspaces(ctx)
		if err != nil {
			return err
		}
		if n > 0 && !in.AllowExisting {
			return errors.New("a workspace already exists; pass --allow-existing to create another one")
		}
		person, err := q.GetPersonByEmail(ctx, email)
		if store.IsNotFound(err) {
			person, err = createPerson(ctx, q, email, strings.TrimSpace(in.Name), in.Locale)
		}
		if err != nil {
			return err
		}
		ws, m, err := createOwnedWorkspace(ctx, q, store.CreateWorkspaceParams{ID: uuid.New(), Name: name}, person.ID)
		if err != nil {
			return err
		}
		res = BootstrapResult{WorkspaceID: ws.ID, MemberID: m.ID, PersonID: person.ID}
		return nil
	})
	return res, err
}

func workspaceName(s string) (string, bool) {
	name := strings.TrimSpace(s)
	return name, name != "" && len([]rune(name)) <= 200
}

// createOwnedWorkspace is the one way a workspace comes to be, from `yuva bootstrap` or open
// sign-up: the workspace and its owner.
func createOwnedWorkspace(ctx context.Context, q *store.Queries, arg store.CreateWorkspaceParams, personID uuid.UUID) (store.Workspace, store.Member, error) {
	ws, err := q.CreateWorkspace(ctx, arg)
	if err != nil {
		return ws, store.Member{}, err
	}
	m, err := q.CreateMember(ctx, store.CreateMemberParams{ID: uuid.New(), WorkspaceID: ws.ID, PersonID: personID, Role: roleOwner})
	return ws, m, err
}
