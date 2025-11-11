# Full SR-IOV Test Suite - Quick Reference Card

**Date:** November 11, 2025  
**Status:** Ready to execute

## The Command

```bash
cd /root/eco-gotests && \
source ~/newlogin.sh 2>/dev/null && \
export GOTOOLCHAIN=auto && \
export SRIOV_DEVICES="cx7anl244:1021:15b3:ens2f0np0,cx6dxanl244:a2d6:15b3:ens7f0np0" && \
timeout 3600 $(go env GOPATH)/bin/ginkgo -v ./tests/sriov/ 2>&1 | tee /tmp/full_test_run_$(date +%s).log
```

## Tmux Setup

### Start Tests
```bash
# Create tmux session
tmux new-session -s sriov-tests

# Paste and run the command above in tmux
# Then press Ctrl+B followed by D to detach
```

### Attach to Running Tests
```bash
tmux attach-session -t sriov-tests
```

### Exit Tmux
```bash
# Press: Ctrl+B then X
# Or type: exit
```

## Monitoring During Execution

### Watch Tests
```bash
tmux attach-session -t sriov-tests
```

### Watch Log in Real-time
```bash
tail -f /tmp/full_test_run_*.log
```

### Check Cluster Health
```bash
./cluster_health_check.sh
```

### View Only Failures
```bash
grep "\[FAILED\]" /tmp/full_test_run_*.log
```

### Monitor Operator
```bash
oc logs -n openshift-sriov-network-operator -f deployment/sriov-network-operator
```

## Expected Timeline

- **Total Duration:** 1-2 hours
- **Initial Setup:** 2-3 minutes
- **Per Test:** 5-25 minutes (varies by test complexity)
- **Cleanup:** 2-3 minutes

## Good Signs

✅ Regular "By" markers appearing  
✅ Consistent "[INFO]" log messages  
✅ Pod creation/deletion activity  
✅ Tests completing normally  

## Warning Signs

⚠️ No output for 5+ minutes  
⚠️ Repeated ERROR messages  
⚠️ Pods stuck in Pending  
⚠️ Network timeouts  

## Critical Issues

❌ Test marked as [FAILED]  
❌ Pod readiness timeout  
❌ Resource creation failure  
❌ Cluster connectivity loss  

## When to Ask for Help

Share these commands' output if you encounter issues:

```bash
# Current test status
tail -30 /tmp/full_test_run_*.log

# Cluster health
./cluster_health_check.sh --output json

# Failed tests
grep "\[FAILED\]" /tmp/full_test_run_*.log

# Operator status
oc get pods -n openshift-sriov-network-operator -o wide

# Test namespaces
oc get ns | grep -E "e2e-|test-"
```

## After Tests Complete

### Review Results
```bash
# Last 50 lines
tail -50 /tmp/full_test_run_*.log

# Count passes/failures
grep -c "\[PASSED\]" /tmp/full_test_run_*.log
grep -c "\[FAILED\]" /tmp/full_test_run_*.log

# Generate final health report
./cluster_health_check.sh --output html > test_final_health.html
```

## Key Information

- **Test Directory:** `/root/eco-gotests/tests/sriov/`
- **Log Location:** `/tmp/full_test_run_<TIMESTAMP>.log`
- **Timeout:** 1 hour (3600 seconds)
- **Tests Count:** 13 SR-IOV tests
- **Parallel Execution:** Serial (one at a time)

## Files Used

- `sriov_reinstall_test.go` - Operator lifecycle tests
- `sriov_lifecycle_test.go` - Component cleanup tests
- `sriov_advanced_scenarios_test.go` - Advanced scenario tests
- `sriov_bonding_test.go` - Bonding configuration tests
- `sriov_operator_networking_test.go` - Networking tests
- `helpers.go` - Common test utilities

## Tips

1. **Don't close the terminal** - Use Ctrl+B then D to detach from tmux
2. **Check regularly** - Attach and check progress every 15-20 minutes
3. **Use the log file** - It captures everything for later analysis
4. **Cluster health check** - Run it during tests to detect issues early
5. **Save the log** - You may need it for analysis or reporting

## Quick Commands Cheatsheet

| What to Do | Command |
|-----------|---------|
| Start tests | `tmux new-session -s sriov-tests` (then paste command) |
| Attach to session | `tmux attach-session -t sriov-tests` |
| Detach from session | `Ctrl+B then D` |
| Watch log | `tail -f /tmp/full_test_run_*.log` |
| Check cluster | `./cluster_health_check.sh` |
| See failures | `grep "\[FAILED\]" /tmp/full_test_run_*.log` |
| Count tests | `grep -c "\[PASSED\]" /tmp/full_test_run_*.log` |
| Final report | `./cluster_health_check.sh --output html > report.html` |

---

**You're ready to start the tests!**

Copy the command above, create a tmux session, and run it.  
I'll be ready to help if you encounter any issues.

**Good luck! 🚀**

