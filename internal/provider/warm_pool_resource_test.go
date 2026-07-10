// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAccWarmPoolResourceLifecycle(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newWarmPoolAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if remaining := server.service.remaining(); len(remaining) > 0 {
				return fmt.Errorf("warm pools were not deleted: %v", remaining)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccWarmPoolResourceConfig(server.URL, 0, 5),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_warm_pool.api", "id", "warm-pool-1"),
					resource.TestCheckResourceAttr("ona_warm_pool.api", "project_id", "project-1"),
					resource.TestCheckResourceAttr("ona_warm_pool.api", "environment_class_id", "class-1"),
					resource.TestCheckResourceAttr("ona_warm_pool.api", "min_size", "0"),
					resource.TestCheckResourceAttr("ona_warm_pool.api", "max_size", "5"),
					resource.TestCheckResourceAttr("ona_warm_pool.api", "organization_id", "org-1"),
					resource.TestCheckResourceAttr("ona_warm_pool.api", "runner_id", "runner-1"),
					resource.TestCheckResourceAttr("ona_warm_pool.api", "snapshot_id", "snapshot-1"),
					resource.TestCheckResourceAttr("data.ona_warm_pool.api", "warm_pool_id", "warm-pool-1"),
					resource.TestCheckResourceAttr("data.ona_warm_pools.project", "warm_pools.#", "1"),
					resource.TestCheckResourceAttr("data.ona_warm_pools.project", "warm_pools.0.id", "warm-pool-1"),
				),
			},
			{
				Config: testAccWarmPoolResourceConfig(server.URL, 0, 5),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "ona_warm_pool.api",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccWarmPoolResourceConfig(server.URL, 1, 3),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_warm_pool.api", "min_size", "1"),
					resource.TestCheckResourceAttr("ona_warm_pool.api", "max_size", "3"),
				),
			},
			{
				PreConfig: func() {
					server.service.updateSizes("warm-pool-1", 2, 4)
				},
				Config: testAccWarmPoolResourceConfig(server.URL, 1, 3),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_warm_pool.api", "min_size", "1"),
					resource.TestCheckResourceAttr("ona_warm_pool.api", "max_size", "3"),
				),
			},
			{
				PreConfig: func() {
					server.service.remove("warm-pool-1")
				},
				Config: testAccWarmPoolResourceConfig(server.URL, 1, 3),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_warm_pool.api", "id", "warm-pool-2"),
					resource.TestCheckResourceAttr("ona_warm_pool.api", "min_size", "1"),
					resource.TestCheckResourceAttr("ona_warm_pool.api", "max_size", "3"),
				),
			},
		},
	})
}

func TestAccWarmPoolCollectionDataSourcePaginationSorting(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newWarmPoolAPIServer(t)
	t.Cleanup(server.Close)

	server.service.put(server.service.newWarmPool("warm-pool-c", "project-1", "class-1", 0, 2))
	server.service.put(server.service.newWarmPool("warm-pool-a", "project-1", "class-1", 0, 2))
	server.service.put(server.service.newWarmPool("warm-pool-b", "project-1", "class-2", 0, 2))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWarmPoolCollectionDataSourceConfig(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.ona_warm_pools.project", "warm_pools.#", "3"),
					resource.TestCheckResourceAttr("data.ona_warm_pools.project", "warm_pools.0.id", "warm-pool-a"),
					resource.TestCheckResourceAttr("data.ona_warm_pools.project", "warm_pools.1.id", "warm-pool-b"),
					resource.TestCheckResourceAttr("data.ona_warm_pools.project", "warm_pools.2.id", "warm-pool-c"),
				),
			},
		},
	})
}

func testAccWarmPoolResourceConfig(host string, minSize int32, maxSize int32) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_warm_pool" "api" {
  project_id           = "project-1"
  environment_class_id = "class-1"
  min_size             = %[2]d
  max_size             = %[3]d
}

data "ona_warm_pool" "api" {
  warm_pool_id = ona_warm_pool.api.id
}

data "ona_warm_pools" "project" {
  project_ids           = [ona_warm_pool.api.project_id]
  environment_class_ids = [ona_warm_pool.api.environment_class_id]
  page_size             = 1
}
`, host, minSize, maxSize)
}

func testAccWarmPoolCollectionDataSourceConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

data "ona_warm_pools" "project" {
  project_ids = ["project-1"]
  page_size   = 1
}
`, host)
}

type warmPoolAPIServer struct {
	*httptest.Server
	service *fakeWarmPoolService
}

func newWarmPoolAPIServer(t *testing.T) *warmPoolAPIServer {
	t.Helper()

	service := &fakeWarmPoolService{
		warmPools: map[string]*v1.WarmPool{},
		now:       time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
	}
	path, handler := v1connect.NewPrebuildServiceHandler(service)
	server := httptest.NewServer(http.StripPrefix("/api", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == path || len(r.URL.Path) > len(path) && r.URL.Path[:len(path)] == path {
			handler.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})))
	return &warmPoolAPIServer{
		Server:  server,
		service: service,
	}
}

type fakeWarmPoolService struct {
	v1connect.UnimplementedPrebuildServiceHandler

	mu        sync.Mutex
	warmPools map[string]*v1.WarmPool
	nextID    int
	now       time.Time
}

func (s *fakeWarmPoolService) CreateWarmPool(ctx context.Context, req *connect.Request[v1.CreateWarmPoolRequest]) (*connect.Response[v1.CreateWarmPoolResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	id := fmt.Sprintf("warm-pool-%d", s.nextID)
	warmPool := s.newWarmPool(id, req.Msg.GetProjectId(), req.Msg.GetEnvironmentClassId(), req.Msg.GetMinSize(), req.Msg.GetMaxSize())
	s.warmPools[id] = warmPool
	return connect.NewResponse(&v1.CreateWarmPoolResponse{WarmPool: cloneWarmPool(warmPool)}), nil
}

func (s *fakeWarmPoolService) GetWarmPool(ctx context.Context, req *connect.Request[v1.GetWarmPoolRequest]) (*connect.Response[v1.GetWarmPoolResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	warmPool := s.warmPools[req.Msg.GetWarmPoolId()]
	if warmPool == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("warm pool not found"))
	}
	return connect.NewResponse(&v1.GetWarmPoolResponse{WarmPool: cloneWarmPool(warmPool)}), nil
}

func (s *fakeWarmPoolService) ListWarmPools(ctx context.Context, req *connect.Request[v1.ListWarmPoolsRequest]) (*connect.Response[v1.ListWarmPoolsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var warmPools []*v1.WarmPool
	for _, warmPool := range s.warmPools {
		if !matchesWarmPoolFilter(warmPool, req.Msg.GetFilter()) {
			continue
		}
		warmPools = append(warmPools, cloneWarmPool(warmPool))
	}
	sort.SliceStable(warmPools, func(i, j int) bool {
		return warmPools[i].GetId() < warmPools[j].GetId()
	})

	offset := 0
	if req.Msg.GetPagination().GetToken() != "" {
		if _, err := fmt.Sscanf(req.Msg.GetPagination().GetToken(), "%d", &offset); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid pagination token: %w", err))
		}
	}
	if offset > len(warmPools) {
		offset = len(warmPools)
	}

	pageSize := int(req.Msg.GetPagination().GetPageSize())
	if pageSize <= 0 {
		pageSize = len(warmPools)
	}

	nextOffset := offset + pageSize
	nextToken := ""
	if nextOffset < len(warmPools) {
		nextToken = fmt.Sprintf("%d", nextOffset)
	} else {
		nextOffset = len(warmPools)
	}
	warmPools = warmPools[offset:nextOffset]

	return connect.NewResponse(&v1.ListWarmPoolsResponse{
		Pagination: &v1.PaginationResponse{NextToken: nextToken},
		WarmPools:  warmPools,
	}), nil
}

func (s *fakeWarmPoolService) UpdateWarmPool(ctx context.Context, req *connect.Request[v1.UpdateWarmPoolRequest]) (*connect.Response[v1.UpdateWarmPoolResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	warmPool := s.warmPools[req.Msg.GetWarmPoolId()]
	if warmPool == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("warm pool not found"))
	}
	if req.Msg.MinSize != nil {
		warmPool.Spec.MinSize = ptr(req.Msg.GetMinSize())
	}
	if req.Msg.MaxSize != nil {
		warmPool.Spec.MaxSize = ptr(req.Msg.GetMaxSize())
	}
	warmPool.Metadata.UpdatedAt = timestamppb.New(s.now.Add(time.Hour))
	warmPool.Status.RunningInstances = warmPool.GetSpec().GetMinSize()
	warmPool.Status.StoppedInstances = warmPool.GetSpec().GetMaxSize() - warmPool.GetSpec().GetMinSize()
	warmPool.Status.DesiredSize = warmPool.GetSpec().GetMaxSize()
	return connect.NewResponse(&v1.UpdateWarmPoolResponse{WarmPool: cloneWarmPool(warmPool)}), nil
}

func (s *fakeWarmPoolService) DeleteWarmPool(ctx context.Context, req *connect.Request[v1.DeleteWarmPoolRequest]) (*connect.Response[v1.DeleteWarmPoolResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.warmPools[req.Msg.GetWarmPoolId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("warm pool not found"))
	}
	delete(s.warmPools, req.Msg.GetWarmPoolId())
	return connect.NewResponse(&v1.DeleteWarmPoolResponse{}), nil
}

func (s *fakeWarmPoolService) newWarmPool(id string, projectID string, environmentClassID string, minSize int32, maxSize int32) *v1.WarmPool {
	return &v1.WarmPool{
		Id: id,
		Metadata: &v1.WarmPoolMetadata{
			OrganizationId:     "org-1",
			ProjectId:          projectID,
			EnvironmentClassId: environmentClassID,
			RunnerId:           "runner-1",
			CreatedAt:          timestamppb.New(s.now),
			UpdatedAt:          timestamppb.New(s.now),
		},
		Spec: &v1.WarmPoolSpec{
			SnapshotId:   ptr("snapshot-1"),
			DesiredPhase: v1.WarmPoolPhase_WARM_POOL_PHASE_READY,
			MinSize:      ptr(minSize),
			MaxSize:      ptr(maxSize),
		},
		Status: &v1.WarmPoolStatus{
			Phase:            v1.WarmPoolPhase_WARM_POOL_PHASE_READY,
			RunningInstances: minSize,
			StoppedInstances: maxSize - minSize,
			DesiredSize:      maxSize,
		},
	}
}

func (s *fakeWarmPoolService) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.warmPools, id)
}

func (s *fakeWarmPoolService) put(warmPool *v1.WarmPool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.warmPools[warmPool.GetId()] = warmPool
}

func (s *fakeWarmPoolService) updateSizes(id string, minSize int32, maxSize int32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	warmPool := s.warmPools[id]
	if warmPool == nil {
		return
	}
	warmPool.Spec.MinSize = ptr(minSize)
	warmPool.Spec.MaxSize = ptr(maxSize)
	warmPool.Status.RunningInstances = minSize
	warmPool.Status.StoppedInstances = maxSize - minSize
	warmPool.Status.DesiredSize = maxSize
}

func (s *fakeWarmPoolService) remaining() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]string, 0, len(s.warmPools))
	for id := range s.warmPools {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func matchesWarmPoolFilter(warmPool *v1.WarmPool, filter *v1.ListWarmPoolsRequest_Filter) bool {
	if filter == nil {
		return true
	}
	if len(filter.GetProjectIds()) > 0 && !contains(filter.GetProjectIds(), warmPool.GetMetadata().GetProjectId()) {
		return false
	}
	if len(filter.GetEnvironmentClassIds()) > 0 && !contains(filter.GetEnvironmentClassIds(), warmPool.GetMetadata().GetEnvironmentClassId()) {
		return false
	}
	return true
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cloneWarmPool(warmPool *v1.WarmPool) *v1.WarmPool {
	if warmPool == nil {
		return nil
	}
	result := &v1.WarmPool{}
	proto.Merge(result, warmPool)
	return result
}

func ptr[T any](value T) *T {
	return &value
}
