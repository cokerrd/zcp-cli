// Package snapshot provides ZCP block storage snapshot API operations
// targeting the STKCNSL API.
package snapshot

import (
	"context"
	"fmt"
	"net/url"

	"github.com/zsoftly/zcp-cli/pkg/httpclient"
)

// Snapshot represents a STKCNSL block storage snapshot.
type Snapshot struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	Slug                 string  `json:"slug"`
	BlockstorageID       string  `json:"blockstorage_id"`
	UserID               string  `json:"user_id"`
	AccountID            string  `json:"account_id"`
	ProjectID            string  `json:"project_id"`
	RegionID             string  `json:"region_id"`
	CloudProviderID      string  `json:"cloud_provider_id"`
	CloudProviderSetupID string  `json:"cloud_provider_setup_id"`
	RequestStatus        bool    `json:"request_status"`
	ServiceName          string  `json:"service_name"`
	ServiceDisplayName   string  `json:"service_display_name"`
	AllTimeConsumption   float64 `json:"all_time_consumption"`
	HasContract          bool    `json:"has_contract"`
	FrozenAt             *string `json:"frozen_at"`
	SuspendedAt          *string `json:"suspended_at"`
	TerminatedAt         *string `json:"terminated_at"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
	DeletedAt            *string `json:"deleted_at"`
}

// listResponse is the STKCNSL paginated envelope for block storage snapshots.
type listResponse struct {
	Status      string     `json:"status"`
	Message     string     `json:"message"`
	CurrentPage int        `json:"current_page"`
	Data        []Snapshot `json:"data"`
	Total       *int       `json:"total"`
}

// singleResponse is used when the API returns a single snapshot in `data`.
type singleResponse struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Data    Snapshot `json:"data"`
}

// CreateRequest holds parameters for creating a block storage snapshot.
type CreateRequest struct {
	Name          string `json:"name"`
	Plan          string `json:"plan"`
	Service       string `json:"service"`
	IsMemory      bool   `json:"is_memory"`
	Project       string `json:"project"`
	CloudProvider string `json:"cloud_provider"`
	Region        string `json:"region"`
	BillingCycle  string `json:"billing_cycle"`
	Coupon        string `json:"coupon,omitempty"`
}

// Service provides block storage snapshot API operations.
type Service struct {
	client *httpclient.Client
}

const maxListPages = 1000

// NewService creates a new snapshot Service.
func NewService(client *httpclient.Client) *Service {
	return &Service{client: client}
}

// List returns block storage snapshots.
func (s *Service) List(ctx context.Context, region, project string) ([]Snapshot, error) {
	q := url.Values{
		"include": {"blockstorage,region,cloud_provider,project"},
	}
	if region != "" {
		q.Set("filter[region]", region)
	}
	if project != "" {
		q.Set("filter[project]", project)
	}
	var all []Snapshot
	var reportedTotal *int
	for page := 1; page <= maxListPages; page++ {
		pageQuery := make(url.Values, len(q))
		for key, values := range q {
			pageQuery[key] = append([]string(nil), values...)
		}
		if page > 1 {
			pageQuery.Set("page", fmt.Sprintf("%d", page))
		}

		var resp listResponse
		if err := s.client.Get(ctx, "/blockstorages/snapshots", pageQuery, &resp); err != nil {
			return nil, fmt.Errorf("listing block storage snapshots: %w", err)
		}
		if resp.CurrentPage < 0 {
			return nil, fmt.Errorf("listing block storage snapshots: API returned invalid negative page %d", resp.CurrentPage)
		}
		if page > 1 && resp.CurrentPage == 0 {
			return nil, fmt.Errorf("listing block storage snapshots: requested page %d but the API did not return pagination metadata", page)
		}
		if resp.CurrentPage > 0 && resp.CurrentPage != page {
			return nil, fmt.Errorf("listing block storage snapshots: requested page %d but the API returned page %d", page, resp.CurrentPage)
		}
		all = append(all, resp.Data...)
		if reportedTotal == nil {
			reportedTotal = resp.Total
		} else if resp.Total == nil || *resp.Total != *reportedTotal {
			return nil, fmt.Errorf("listing block storage snapshots: API changed reported total on page %d", page)
		}
		if reportedTotal == nil {
			return all, nil
		}
		if *reportedTotal < 0 {
			return nil, fmt.Errorf("listing block storage snapshots: API returned invalid negative total %d", *reportedTotal)
		}
		if *reportedTotal == 0 {
			if len(resp.Data) > 0 {
				return nil, fmt.Errorf("listing block storage snapshots: page %d returned snapshots despite a total of zero", page)
			}
			return all, nil
		}
		if len(all) > *reportedTotal {
			return nil, fmt.Errorf("listing block storage snapshots: page %d produced more snapshots than the reported total %d", page, *reportedTotal)
		}
		if len(resp.Data) == 0 {
			return nil, fmt.Errorf("listing block storage snapshots: page %d was empty before reaching reported total %d", page, *reportedTotal)
		}
		if len(all) == *reportedTotal {
			return all, nil
		}
	}
	return nil, fmt.Errorf("listing block storage snapshots: exceeded %d pages without reaching the reported total", maxListPages)
}

// Create creates a new block storage snapshot.
func (s *Service) Create(ctx context.Context, blockstorageSlug string, req CreateRequest) (*Snapshot, error) {
	var resp singleResponse
	path := fmt.Sprintf("/blockstorages/%s/snapshots", blockstorageSlug)
	if err := s.client.Post(ctx, path, req, &resp); err != nil {
		return nil, fmt.Errorf("creating block storage snapshot: %w", err)
	}
	return &resp.Data, nil
}

// Revert reverts a block storage volume to a snapshot state.
func (s *Service) Revert(ctx context.Context, blockstorageSlug, snapshotSlug string) (*Snapshot, error) {
	var resp singleResponse
	path := fmt.Sprintf("/blockstorages/%s/snapshots/%s/revert", blockstorageSlug, snapshotSlug)
	if err := s.client.Post(ctx, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("reverting block storage snapshot %s: %w", snapshotSlug, err)
	}
	return &resp.Data, nil
}

// Delete permanently deletes a block storage snapshot.
func (s *Service) Delete(ctx context.Context, slug string) error {
	if err := s.client.Delete(ctx, "/blockstorages/snapshots/"+slug, nil); err != nil {
		return fmt.Errorf("deleting block storage snapshot %s: %w", slug, err)
	}
	return nil
}
