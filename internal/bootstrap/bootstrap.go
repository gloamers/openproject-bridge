package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/gloamers/openproject-bridge/internal/config"
	"github.com/gloamers/openproject-bridge/internal/opclient"
)

// Status of an ensure step.
type Status string

const (
	StatusExists  Status = "exists"
	StatusCreated Status = "created"
	StatusWould   Status = "would-create"
)

// Row is one line of the bootstrap report.
type Row struct {
	OrgID      string
	Identifier string
	OPID       int
	GitHub     string
	Kind       string // parent|product
	Status     Status
}

// ProjectsAPI is the subset of opclient used by bootstrap (testable).
type ProjectsAPI interface {
	GetByIdentifier(ctx context.Context, identifier string) (*opclient.Project, error)
	CreateProject(ctx context.Context, in opclient.CreateProjectInput) (*opclient.Project, error)
}

// Options configure EnsureOrganization.
type Options struct {
	DryRun bool
}

// EnsureOrganization creates parent + products idempotently.
func EnsureOrganization(ctx context.Context, api ProjectsAPI, org *config.Organization, opts Options) ([]Row, error) {
	var rows []Row

	parent, st, err := ensureProject(ctx, api, opts.DryRun, opclient.CreateProjectInput{
		Name:        org.Parent.Name,
		Identifier:  org.Parent.Identifier,
		Description: org.Parent.Description,
		Public:      org.Parent.Public,
	})
	if err != nil {
		return rows, fmt.Errorf("bootstrap: org %q parent: %w", org.ID, err)
	}
	rows = append(rows, Row{
		OrgID:      org.ID,
		Identifier: org.Parent.Identifier,
		OPID:       idOf(parent),
		Kind:       "parent",
		Status:     st,
	})

	parentID := idOf(parent)
	if parentID == 0 && !opts.DryRun {
		return rows, fmt.Errorf("bootstrap: org %q: parent id missing after ensure", org.ID)
	}

	for _, child := range org.Products {
		in := opclient.CreateProjectInput{
			Name:        child.Name,
			Identifier:  child.Identifier,
			Description: child.Description,
			Public:      false,
			ParentID:    parentID,
		}
		if opts.DryRun && parentID == 0 {
			in.ParentID = 0
		}
		p, st, err := ensureProject(ctx, api, opts.DryRun, in)
		if err != nil {
			return rows, fmt.Errorf("bootstrap: org %q product %q: %w", org.ID, child.Identifier, err)
		}
		rows = append(rows, Row{
			OrgID:      org.ID,
			Identifier: child.Identifier,
			OPID:       idOf(p),
			GitHub:     child.GitHub,
			Kind:       "product",
			Status:     st,
		})
	}
	return rows, nil
}

func ensureProject(ctx context.Context, api ProjectsAPI, dryRun bool, in opclient.CreateProjectInput) (*opclient.Project, Status, error) {
	existing, err := api.GetByIdentifier(ctx, in.Identifier)
	if err == nil {
		return existing, StatusExists, nil
	}
	if !errors.Is(err, opclient.ErrNotFound) {
		return nil, "", err
	}
	if dryRun {
		return &opclient.Project{Identifier: in.Identifier, Name: in.Name}, StatusWould, nil
	}
	created, err := api.CreateProject(ctx, in)
	if err != nil {
		return nil, "", err
	}
	return created, StatusCreated, nil
}

func idOf(p *opclient.Project) int {
	if p == nil {
		return 0
	}
	return p.ID
}

// WriteReport prints a tab-separated table to w.
func WriteReport(w io.Writer, rows []Row) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "ORG\tKIND\tIDENTIFIER\tOP_ID\tGITHUB\tSTATUS"); err != nil {
		return err
	}
	for _, r := range rows {
		opid := "-"
		if r.OPID > 0 {
			opid = fmt.Sprintf("%d", r.OPID)
		}
		gh := r.GitHub
		if gh == "" {
			gh = "-"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			r.OrgID, r.Kind, r.Identifier, opid, gh, r.Status); err != nil {
			return err
		}
	}
	return tw.Flush()
}
