package ai

import "testing"

func TestNormalizeIdentityAddsDiscoverableOrderWorkflow(t *testing.T) {
	id := &Identity{
		ServerName:  "orders-prod",
		Description: "order server",
		Persona:     "Fulfillment API for production orders.",
		Tools:       []ToolDef{{Name: "lookup_customer", Description: "Lookup a customer.", Params: []string{"customer_id"}}},
	}

	normalizeIdentity(id)

	if !hasTool(id.Tools, "get_orders") {
		t.Fatal("expected get_orders to be added")
	}
	setOrderType, ok := findTool(id.Tools, "set_order_type")
	if !ok {
		t.Fatal("expected set_order_type to be added")
	}
	if len(setOrderType.Params) != 2 || setOrderType.Params[0] != "order_id" || setOrderType.Params[1] != "order_type" {
		t.Fatalf("unexpected set_order_type params: %#v", setOrderType.Params)
	}
}

func TestEnsureOrderWorkflowDoesNotDuplicateExistingTools(t *testing.T) {
	tools := []ToolDef{
		{Name: "get_orders", Description: "List orders."},
		{Name: "set_order_type", Description: "Set order type.", Params: []string{"order_id", "order_type"}},
	}

	got := ensureOrderWorkflow(tools)

	if countTool(got, "get_orders") != 1 {
		t.Fatalf("expected one get_orders, got %d", countTool(got, "get_orders"))
	}
	if countTool(got, "set_order_type") != 1 {
		t.Fatalf("expected one set_order_type, got %d", countTool(got, "set_order_type"))
	}
}

func hasTool(tools []ToolDef, name string) bool {
	_, ok := findTool(tools, name)
	return ok
}

func findTool(tools []ToolDef, name string) (ToolDef, bool) {
	for _, tool := range tools {
		if tool.Name == name {
			return tool, true
		}
	}
	return ToolDef{}, false
}

func countTool(tools []ToolDef, name string) int {
	count := 0
	for _, tool := range tools {
		if tool.Name == name {
			count++
		}
	}
	return count
}
