# SR-IOV Operator Incomplete NAD Bug - Investigation Summary

**Status**: ✅ COMPLETE  
**Date**: November 12, 2025  
**Severity**: CRITICAL - Blocks SR-IOV networking  
**Root Cause**: Template placement logic bug

---

## Executive Summary

The SR-IOV operator creates NetworkAttachmentDefinition (NAD) resources with **incomplete configuration**. When user pods try to attach to SR-IOV networks, the CNI plugin fails because critical fields (`resourceName` and `pciAddress`) are missing from the NAD's spec.config JSON (though `resourceName` is correctly placed in metadata.annotations).

**Result**: Pods remain in Pending state with error: "SRIOV-CNI failed to load netconf: LoadConf(): VF pci addr is required"

---

## Quick Navigation

| Document | Purpose | Read Time |
|----------|---------|-----------|
| **1. This file** | Overview & quick facts | 5 min |
| **2. ROOT_CAUSE_AND_CODE_ANALYSIS.md** | Complete code analysis | 15 min |
| **3. BUG_EVIDENCE_AND_REPRODUCTION.md** | Proof & reproduction tools | 10 min |
| **4. UPSTREAM_BUG_REPORT.md** | Ready to file upstream | 10 min |
| **5. DOCUMENTATION_SUMMARY.md** | Key clarifications | 5 min |
| **6. CONTEXT_AND_USAGE_GUIDE.md** | When/why/how bug manifests | 10 min |

---

## Problem at a Glance

### What Goes Wrong

```
Expected NAD spec.config:
{
  "resourceName": "openshift.io/cx7anl244",  ✅ CNI needs this
  "pciAddress": "0000:02:01.2",              ✅ CNI needs this
  "type": "sriov"
}

Actual NAD spec.config:
{
  "type": "sriov"
  # ❌ resourceName MISSING!
  # ❌ pciAddress MISSING!
}

BUT: resourceName IS in metadata.annotations ✅
```

### Impact

- ❌ Pod attachment fails
- ❌ All SR-IOV networking broken
- ❌ Tests timeout waiting for pod readiness
- ✅ Only manifests when creating NEW networks or after operator restart

---

## When Bug Manifests

**NOT in normal operation** (pre-configured networks work fine)

**YES in these situations:**
1. Creating a NEW SriovNetwork resource
2. After operator restart/reinstallation
3. Comprehensive testing (like your tests!)
4. When NADs are regenerated

**Why**: Operator only creates NAD when you create SriovNetwork resource

---

## Root Cause

**Location**: `bindata/manifests/cni-config/sriov/` (template files)

**Issue**: Template placement logic puts `resourceName` in annotations but NOT in spec.config JSON

**Go Code**: ✅ CORRECT - properly prepares `data.Data["CniResourceName"]`

**Templates**: ❌ BUGGY - uses it in annotations but not in CNI config

---

## Evidence

### From Actual Operator Logs

```
Rendered NAD:
{
  "metadata": {
    "annotations": {
      "k8s.v1.cni.cncf.io/resourceName": "openshift.io/test-sriov-nic"  ✅ HERE
    }
  },
  "spec": {
    "config": "{ \"cniVersion\":\"1.0.0\", \"type\":\"sriov\", ... }"
                                                            ❌ NOT HERE!
  }
}
```

### Pod Attachment Flow

```
1. Pod tries to attach to network
2. CNI plugin reads spec.config JSON
3. Looks for "resourceName" field
4. ❌ NOT FOUND (in annotations, not config!)
5. Looks for "pciAddress" field
6. ❌ NOT FOUND either
7. ERROR: "VF pci addr is required"
8. Pod fails to become ready
```

---

## The Discovery

**Initial Question**: Why doesn't operator create complete NAD?

**Investigation**: Found resourceName IS in output (annotations), just misplaced

**Corrected Understanding**: This is a TEMPLATE PLACEMENT BUG, not a missing feature

**Why This Matters**: Shows operator KNOWS about resourceName, but template logic puts it in wrong location

---

## Testing Context

Your tests catch this bug because they:
- ✅ Create fresh SriovNetwork resources (triggers NAD generation)
- ✅ Actually use the networks (tries pod attachment)
- ✅ Test complete lifecycle (restart, reinstall, etc.)
- ✅ Comprehensive testing exposes edge cases

Production deployments often DON'T catch this because they use pre-configured, static networks.

---

## Fix Strategy

**Add missing fields to spec.config JSON in templates:**

```json
{
  "cniVersion": "1.0.0",
  "name": "{{ .NetworkName }}",
  "type": "sriov",
  "resourceName": "{{ .CniResourceName }}",  ← ADD THIS
  "pciAddress": "{{ .PciAddress }}",         ← ADD THIS
  // ... other fields ...
}
```

**Keep the existing:**
```yaml
metadata:
  annotations:
    k8s.v1.cni.cncf.io/resourceName: "{{ .CniResourceName }}"  ← ALREADY CORRECT
```

---

## Key Facts

| Aspect | Details |
|--------|---------|
| **Bug Type** | Template placement logic |
| **What's Missing** | resourceName & pciAddress in spec.config JSON |
| **What's Present** | resourceName in metadata.annotations (correct) |
| **Go Code** | ✅ CORRECT |
| **Template Files** | ❌ BUGGY |
| **First Found** | November 12, 2025 |
| **Status** | Root cause identified, evidence collected |
| **Ready to File** | ✅ YES |

---

## How to Use This Package

1. **Quick Understanding** (15 min):
   - Read this file
   - Read `CONTEXT_AND_USAGE_GUIDE.md`
   - Read `DOCUMENTATION_SUMMARY.md`

2. **Complete Analysis** (45 min):
   - Add `ROOT_CAUSE_AND_CODE_ANALYSIS.md`
   - Add `BUG_EVIDENCE_AND_REPRODUCTION.md`

3. **File Upstream**:
   - Use `UPSTREAM_BUG_REPORT.md`
   - Include `reproduce_incomplete_nad_bug.sh` script
   - Reference the logs in `bug_evidence/` directory

---

## Summary

| Item | Status |
|------|--------|
| **Root Cause** | ✅ IDENTIFIED - template placement bug |
| **Code Location** | ✅ VERIFIED - main branch confirmed |
| **Evidence** | ✅ COLLECTED - actual operator logs |
| **Reproduction** | ✅ AUTOMATED - script included |
| **Analysis** | ✅ COMPLETE - full investigation done |
| **Ready to File** | ✅ YES - all materials prepared |

---

## Next Steps

Choose your path:

**Path 1: Understand the Bug (30 min)**
→ Read files 1-6 sequentially

**Path 2: File Upstream (20 min)**
→ Read `UPSTREAM_BUG_REPORT.md`
→ Include reproduction script
→ File with logs

**Path 3: Deep Dive (60 min)**
→ Focus on `ROOT_CAUSE_AND_CODE_ANALYSIS.md`
→ Study the source code
→ Review operator logs

---

*Package created: November 12, 2025*  
*Last updated: Consolidation to 6 core documents*  
*Status: Ready for upstream filing*

