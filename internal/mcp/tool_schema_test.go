package mcp

import (
	"strings"
	"testing"

	"github.com/riza/wasphole/internal/ai"
)

func TestParamDescriptionPointsSetOrderTypeToGetOrders(t *testing.T) {
	tool := ai.ToolDef{Name: "set_order_type"}

	orderID := paramDescription(tool, "order_id")
	if !strings.Contains(orderID, "get_orders") {
		t.Fatalf("expected order_id description to mention get_orders, got %q", orderID)
	}

	orderType := paramDescription(tool, "order_type")
	if !strings.Contains(orderType, "get_orders") {
		t.Fatalf("expected order_type description to mention get_orders, got %q", orderType)
	}
}
