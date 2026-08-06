package awscosts

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/costexplorer"
)

// Service is the RPC-facing AWS Costs backend (no tview).
type Service struct {
	mu          sync.Mutex
	client      *AWSCostClient
	profile     string
	region      string
	accessKey   string
	secretKey   string
	granularity string
	timeRange   string
	costData    []*CostData
	currentView string
}

// NewService creates an AWS Costs RPC service.
func NewService() *Service {
	return &Service{
		granularity: "DAILY",
		timeRange:   "LAST_30_DAYS",
		currentView: awsViewMain,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "awsCosts",
		Version:     "1.0.0",
		Description: "AWS Cost Explorer and Budget Analyzer",
		Author:      "OhMyOps",
		License:     "MIT",
		Tags:        []string{"aws", "cost", "monitoring", "billing"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("Service.Configure begin")

	if req.Settings == nil {
		return fmt.Errorf("missing settings")
	}
	s.profile = req.Settings["name"]
	s.accessKey = req.Settings["username"]
	s.secretKey = req.Settings["password"]
	s.region = req.Settings["region"]
	if s.region == "" {
		s.region = req.Settings["host"]
	}
	if s.region == "" {
		s.region = req.Settings["url"]
	}
	if s.region == "" {
		s.region = "us-east-1"
	}
	if s.profile == "" && s.accessKey == "" {
		return fmt.Errorf("name (profile) or username (access key) required")
	}
	pluginrpc.RPCLog("Service.Configure profile=%s region=%s", s.profile, s.region)
	s.client = nil
	s.costData = nil
	return nil
}

func (s *Service) GetView(req pluginrpc.ViewRequest) (pluginrpc.ViewData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	viewID := req.View
	if viewID == "" {
		viewID = s.currentView
	}
	if viewID == "" {
		viewID = awsViewMain
	}
	pluginrpc.RPCLog("Service.GetView begin view=%s", viewID)
	start := time.Now()
	view, err := s.buildViewLocked(viewID)
	if err != nil {
		pluginrpc.RPCLog("Service.GetView err=%v", err)
		return pluginrpc.ViewData{}, err
	}
	pluginrpc.RPCLog("Service.GetView OK view=%s rows=%d dur=%s", view.View, len(view.Rows), time.Since(start))
	return view, nil
}

func (s *Service) DoAction(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	action := req.Action
	if strings.HasPrefix(action, "goto_") {
		viewID := strings.TrimPrefix(action, "goto_")
		view, err := s.buildViewLocked(viewID)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "switched to " + viewID, Next: &view}, nil
	}

	switch action {
	case "refresh", "":
		s.costData = nil // force reload
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "refreshed", Next: &view}, nil

	case "toggle_granularity":
		if s.granularity == "DAILY" {
			s.granularity = "MONTHLY"
		} else {
			s.granularity = "DAILY"
		}
		s.costData = nil
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "granularity: " + s.granularity, Next: &view}, nil

	case "set_time_period":
		period := req.Payload["period"]
		if period == "" {
			period = req.Payload["key"]
		}
		period = strings.TrimSpace(strings.TrimSuffix(period, " (current)"))
		periods := []string{
			"LAST_7_DAYS", "LAST_30_DAYS", "THIS_MONTH",
			"LAST_3_MONTHS", "LAST_6_MONTHS", "LAST_12_MONTHS",
		}
		valid := false
		for _, p := range periods {
			if p == period {
				valid = true
				break
			}
		}
		if !valid {
			// Cycle to next period when host has no period picker yet.
			idx := 0
			for i, p := range periods {
				if p == s.timeRange {
					idx = (i + 1) % len(periods)
					break
				}
			}
			period = periods[idx]
		}
		s.timeRange = period
		s.costData = nil
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "period: " + period, Next: &view}, nil

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = nil
	s.costData = nil
	return nil
}

func (s *Service) ensureClientLocked() error {
	if s.client != nil {
		return nil
	}
	if s.profile == "" && s.accessKey == "" {
		return fmt.Errorf("not configured")
	}
	client, err := NewAWSCostClientWithCreds(s.profile, s.region, s.accessKey, s.secretKey)
	if err != nil {
		return err
	}
	s.client = client
	return nil
}

func (s *Service) loadCostDataLocked() error {
	if s.costData != nil {
		return nil
	}
	if err := s.ensureClientLocked(); err != nil {
		return err
	}
	tr := getCostTimeRange(s.timeRange)
	out, err := s.client.GetCostsByService(tr.Start, tr.End, s.granularity)
	if err != nil {
		return err
	}
	s.costData = processCostExplorerOutput(out, s.region)
	return nil
}

func processCostExplorerOutput(costData *costexplorer.GetCostAndUsageOutput, region string) []*CostData {
	result := make([]*CostData, 0)
	for _, resultByTime := range costData.ResultsByTime {
		for _, group := range resultByTime.Groups {
			serviceName := aws.StringValue(group.Keys[0])
			cost := 0.0
			unit := "USD"
			if amount, ok := group.Metrics["BlendedCost"]; ok && amount.Amount != nil {
				if parsed, err := strconv.ParseFloat(aws.StringValue(amount.Amount), 64); err == nil {
					cost = parsed
				}
				if amount.Unit != nil {
					unit = aws.StringValue(amount.Unit)
				}
			}
			result = append(result, &CostData{
				service:   serviceName,
				cost:      cost,
				date:      aws.StringValue(resultByTime.TimePeriod.Start),
				unit:      unit,
				trend:     0,
				forecast:  cost,
				budget:    0,
				region:    region,
				usageType: "Usage",
			})
		}
	}
	return result
}
