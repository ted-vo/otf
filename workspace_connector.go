package otf

import (
	"context"

	"github.com/leg100/otf/cloud"
	"github.com/pkg/errors"
)

// WorkspaceConnector connects a workspace to a VCS repo, subscribing it to
// VCS events that trigger runs.
type WorkspaceConnector struct {
	Application
	*WebhookCreator
	*WebhookUpdater
}

type ConnectWorkspaceOptions struct {
	Identifier string `schema:"identifier,required"` // repo id: <owner>/<repo>
	ProviderID string `schema:"vcs_provider_id,required"`
	Cloud      string // cloud host of the repo
}

func (wc *WorkspaceConnector) Connect(ctx context.Context, workspaceID string, opts ConnectWorkspaceOptions) (*Workspace, error) {
	repo, err := wc.GetRepository(ctx, opts.ProviderID, opts.Identifier)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving repository info")
	}

	// Inside transaction:
	// 1. synchronise webhook config
	// 2. create workspace repo in store
	var ws *Workspace
	err = wc.Tx(ctx, func(app Application) (err error) {
		webhook, err := app.DB().SyncWebhook(ctx, SyncWebhookOptions{
			Identifier:        opts.Identifier,
			ProviderID:        opts.ProviderID,
			Cloud:             opts.Cloud,
			CreateWebhookFunc: wc.Create,
			UpdateWebhookFunc: wc.Update,
		})
		if err != nil {
			return errors.Wrap(err, "syncing webhook")
		}

		ws, err = app.DB().CreateWorkspaceRepo(ctx, workspaceID, WorkspaceRepo{
			Branch:     repo.Branch,
			ProviderID: opts.ProviderID,
			WebhookID:  webhook.WebhookID,
		})
		return errors.Wrap(err, "creating workspace repo")
	})
	if err != nil {
		return nil, errors.Wrap(err, "transaction error")
	}
	return ws, nil
}

// Disconnect a repo from a workspace. The repo's webhook is deleted if no other
// workspace is connected to the repo.
func (wc *WorkspaceConnector) Disconnect(ctx context.Context, workspaceID string) (*Workspace, error) {
	// Perform three operations within a transaction:
	// 1. delete workspace repo from db
	// 2. delete webhook from db
	// 3. delete webhook from vcs provider
	var ws *Workspace
	err := wc.Tx(ctx, func(app Application) (err error) {
		ws, err = app.DB().GetWorkspace(ctx, workspaceID)
		if err != nil {
			return err
		}
		repo := ws.Repo()

		ws, err = app.DB().DeleteWorkspaceRepo(ctx, workspaceID)
		if err != nil {
			return err
		}

		hook, err := app.DB().GetWebhook(ctx, repo.WebhookID)
		if err != nil {
			return err
		}

		err = app.DB().DeleteWebhook(ctx, repo.WebhookID)
		if errors.Is(err, ErrForeignKeyViolation) {
			// webhook is still in use by another workspace
			return nil
		} else if err != nil {
			return err
		}

		err = app.DeleteWebhook(ctx, repo.ProviderID, cloud.DeleteWebhookOptions{
			Identifier: repo.Identifier,
			ID:         hook.VCSID,
		})
		if err != nil {
			return err
		}
		return nil
	})
	return ws, err
}