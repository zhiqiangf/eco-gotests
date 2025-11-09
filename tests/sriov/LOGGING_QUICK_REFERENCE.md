# SR-IOV Logging Enhancement - Quick Reference

## File Implementation Checklist

### Phase 1: Complete First
- [ ] **sriov_reinstall_test.go** (~80-100 lines to add)
  - [ ] BeforeEach: Add operator and node discovery logging
  - [ ] test_sriov_operator_control_plane_before_removal: Add phase markers
  - [ ] test_sriov_operator_data_plane_before_removal: Add device/pod creation logging
  - [ ] test_sriov_operator_reinstallation_functionality: Add phase markers (Phase 4 already done)
  - Commit when complete

- [ ] **sriov_lifecycle_test.go** (~150-200 lines to add)
  - [ ] BeforeEach: Add operator and node discovery logging
  - [ ] test_sriov_components_cleanup_on_removal: Add all 4 phase markers + details
  - [ ] test_sriov_resource_deployment_dependency: Add all 5 phase markers + details
  - Commit when complete

### Phase 2: Complete After Phase 1
- [ ] **sriov_advanced_scenarios_test.go** (~200-250 lines to add)
  - [ ] BeforeAll: Add worker node and operator verification logging
  - [ ] test_sriov_end_to_end_telco_scenario: Add all 4 phase markers
  - [ ] test_sriov_multi_feature_integration: Add feature test markers
  - Commit when complete

- [ ] **sriov_bonding_test.go** (~150-200 lines to add)
  - [ ] BeforeAll: Add initialization logging
  - [ ] test_sriov_bond_ipam_integration: Add bonding setup logging
  - [ ] test_sriov_bond_mode_operator_level: Add mode-specific logging
  - Commit when complete

### Phase 3: Complete After Phase 2
- [ ] **sriov_operator_networking_test.go** (~150-200 lines to add)
  - [ ] BeforeAll: Add initialization logging
  - [ ] test_sriov_operator_ipv4_functionality: Add IPv4-specific logging
  - [ ] test_sriov_operator_ipv6_functionality: Add IPv6-specific logging
  - [ ] test_sriov_operator_dual_stack_functionality: Add dual-stack logging
  - Commit when complete

## Copy-Paste Templates

### Template 1: Phase Marker with Info
```go
By("PHASE X: Description of what this phase does")
GinkgoLogr.Info("Starting phase X", "phase", X, "description", "description")
```

### Template 2: Resource Creation
```go
By("Creating resource")
GinkgoLogr.Info("Resource created", "name", resourceName, "namespace", namespace)
GinkgoLogr.Info("Equivalent oc command", "command", 
  fmt.Sprintf("oc get <resource> %s -n %s -o yaml", resourceName, namespace))
```

### Template 3: Verification
```go
By("Verifying something")
GinkgoLogr.Info("Verification complete", "status", statusValue)
```

### Template 4: Error Logging
```go
if err != nil {
    GinkgoLogr.Info("Operation failed", "operation", "operationName", "error", err)
}
```

### Template 5: BeforeEach Setup
```go
GinkgoLogr.Info("SR-IOV operator verified", "namespace", sriovOpNs)

By("Discovering worker nodes")

GinkgoLogr.Info("Worker nodes discovered", "count", len(workerNodes))
```

## Common Locations to Add Logging

1. **After BeforeEach/BeforeAll completion** → Log initialization status
2. **At start of each test phase** → Add `By()` marker
3. **After resource creation** → Log resource info + equivalent oc command
4. **Before verification steps** → Add `By()` marker
5. **During error handling** → Log error with context
6. **Before test cleanup** → Add logging in defer blocks

## Line Count Estimates Per File

| File | Estimated Lines | Estimated By() Calls | Estimated Info() Calls |
|------|-----------------|---------------------|------------------------|
| sriov_reinstall_test.go | 80-100 | 8-10 | 15-20 |
| sriov_lifecycle_test.go | 150-200 | 12-15 | 25-30 |
| sriov_advanced_scenarios_test.go | 200-250 | 15-20 | 30-40 |
| sriov_bonding_test.go | 150-200 | 10-12 | 20-25 |
| sriov_operator_networking_test.go | 150-200 | 10-12 | 20-25 |
| **TOTAL** | **730-950** | **55-69** | **110-140** |

## Git Commit Messages Template

### Phase 1 Commit
```
feat(sriov): Add comprehensive logging to reinstall and lifecycle tests

- Add By() markers for all test phases and major operations
- Add GinkgoLogr.Info() for configuration and resource tracking
- Add equivalent oc commands for manual troubleshooting
- Improve diagnostic visibility in reinstall and lifecycle tests

Affected tests:
- test_sriov_operator_control_plane_before_removal
- test_sriov_operator_data_plane_before_removal
- test_sriov_operator_reinstallation_functionality
- test_sriov_components_cleanup_on_removal
- test_sriov_resource_deployment_dependency
```

### Phase 2 Commit
```
feat(sriov): Add comprehensive logging to advanced scenarios and bonding tests

- Add By() markers and phase descriptions for all tests
- Add structured logging for bonding configuration and operations
- Add equivalent oc commands for advanced scenario troubleshooting
- Improve visibility into complex multi-feature test scenarios

Affected tests:
- test_sriov_end_to_end_telco_scenario
- test_sriov_multi_feature_integration
- test_sriov_bond_ipam_integration
- test_sriov_bond_mode_operator_level
```

### Phase 3 Commit
```
feat(sriov): Add comprehensive logging to networking tests

- Add By() markers and phase descriptions for all tests
- Add IPv4, IPv6, and dual-stack specific logging
- Add equivalent oc commands for address allocation verification
- Complete logging enhancement across all SR-IOV tests

Affected tests:
- test_sriov_operator_ipv4_functionality
- test_sriov_operator_ipv6_functionality
- test_sriov_operator_dual_stack_functionality
```

## Verification Commands

After modifying each file:

```bash
# Format the file
gofmt -w tests/sriov/<filename>.go

# Count logging statements added
grep -c "GinkgoLogr.Info\|By(" tests/sriov/<filename>.go

# Check for syntax errors
go build ./tests/sriov/...

# View the changes
git diff tests/sriov/<filename>.go
```

## Key Points to Remember

✅ **Consistency:** Match the style and formatting of existing logging in `sriov_basic_test.go`
✅ **Context:** Always include relevant variables in log messages
✅ **Commands:** Add equivalent oc commands for major operations
✅ **Organization:** Group logging with related `By()` statements
✅ **Testing:** Run `gofmt` after each file to maintain formatting
✅ **Commits:** One commit per file or per logical group

## Need Help?

Refer to the full **LOGGING_ENHANCEMENT_GUIDE.md** for:
- Complete file-by-file locations
- Exact code snippets for each location
- Detailed explanations of each logging pattern
- Implementation guidelines

## Status Tracking

Use this to track implementation progress:

Phase 1:
- [ ] sriov_reinstall_test.go committed
- [ ] sriov_lifecycle_test.go committed

Phase 2:
- [ ] sriov_advanced_scenarios_test.go committed
- [ ] sriov_bonding_test.go committed

Phase 3:
- [ ] sriov_operator_networking_test.go committed

Final:
- [ ] All 5 files logged and committed
- [ ] `git log` shows 3 phase commits
- [ ] All tests build successfully
- [ ] Ready for PR review

---

**Total Time Estimate:** 2-4 hours for complete implementation (can be done incrementally)

**Recommendation:** Implement one file per session for best focus and clarity.

