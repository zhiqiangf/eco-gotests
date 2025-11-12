# Upstream Bug Report - Ready to File

**Status**: ✅ READY FOR UPSTREAM FILING  
**Date**: November 12, 2025  
**Severity**: CRITICAL

---

## Bug Title

**SR-IOV NetworkAttachmentDefinition Missing resourceName and pciAddress in CNI Config**

---

## Description

The SR-IOV operator's `RenderNetAttDef()` function in `api/v1/helper.go` correctly prepares render data including `CniResourceName`, but the template files in `bindata/manifests/cni-config/sriov/` generate incomplete NetworkAttachmentDefinition (NAD) resources.

### Specific Issue

The generated NAD has `resourceName` placed in `metadata.annotations` (correct for Kubernetes tracking) but MISSING from `spec.config` JSON (required by SR-IOV CNI plugin).

### Result

When pods attempt to attach to SR-IOV networks, the CNI plugin fails with:
```
SRIOV-CNI failed to load netconf: LoadConf(): VF pci addr is required
```

Pods remain in `Pending` state indefinitely.

---

## Affected Versions

- Main branch (latest)
- Any version with incomplete NAD templates

---

## Steps to Reproduce

1. **Create SriovNetworkNodePolicy** to enable VFs on NICs
2. **Create new SriovNetwork** resource
3. **Create test pod** requesting SR-IOV network attachment
4. **Observe**: Pod fails with "VF pci addr is required"

**Automated reproduction**: Use `reproduce_incomplete_nad_bug.sh`

---

## Expected Behavior

NAD should include resourceName and pciAddress in spec.config JSON:

```json
{
  "cniVersion": "0.4.0",
  "name": "network-name",
  "type": "sriov",
  "resourceName": "openshift.io/device-name",
  "pciAddress": "0000:02:01.2",
  "vlan": 0,
  "ipam": {"type": "static"}
}
```

---

## Actual Behavior

NAD spec.config is incomplete:

```json
{
  "cniVersion": "1.0.0",
  "name": "network-name",
  "type": "sriov",
  "vlan": 0,
  "vlanQoS": 0,
  "logLevel": "info",
  "ipam": {"type": "static"}
}
```

Missing: `resourceName`, `pciAddress`

---

## Root Cause

**File**: `bindata/manifests/cni-config/sriov/` (template files)

**Issue**: Template placement logic puts `resourceName` in metadata.annotations but not in spec.config JSON

**Go Code Status**: ✅ CORRECT - properly prepares data

**Template Status**: ❌ BUGGY - misplaced field usage

---

## Evidence

### 1. Operator Logs
```
"render NetworkAttachmentDefinition output"
{
  "spec": {
    "config": "{\"cniVersion\":\"1.0.0\",..., \"ipam\":{\"type\":\"static\"}}"
    # Missing: resourceName, pciAddress
  },
  "metadata": {
    "annotations": {
      "k8s.v1.cni.cncf.io/resourceName": "openshift.io/device"  ← HERE
    }
  }
}
```

### 2. Pod Failure Events
```
FailedCreatePodSandbox:
  SRIOV-CNI failed to load netconf: LoadConf(): VF pci addr is required
```

### 3. Source Code Verification
- File: `api/v1/helper.go`
- Function: `RenderNetAttDef()` for SriovNetwork
- Status: ✅ Correctly prepares `data.Data["CniResourceName"]`
- Location: [github.com/openshift/sriov-network-operator/blob/main/api/v1/helper.go](https://github.com/openshift/sriov-network-operator/blob/main/api/v1/helper.go)

---

## Impact

- **Severity**: CRITICAL
- **Scope**: All SR-IOV networking
- **Manifestation**: Happens when creating NEW networks or after operator restart
- **Workaround**: Use pre-configured networks (stateful approach)

---

## Proposed Fix

### In Template Files (bindata/manifests/cni-config/sriov/)

**Add missing fields to spec.config JSON:**

```yaml
spec:
  config: |
    {
      "cniVersion": "0.4.0",
      "name": "{{ .NetworkName }}",
      "type": "sriov",
      "resourceName": "{{ .CniResourceName }}",  ← ADD THIS
      "pciAddress": "{{ .PciAddress }}",         ← ADD THIS
      "vlan": {{ .Vlan }},
      "ipam": {{ .CniIpam }}
    }
```

**Keep existing** (no change needed):
```yaml
metadata:
  annotations:
    k8s.v1.cni.cncf.io/resourceName: "{{ .CniResourceName }}"
```

---

## Files Affected

- `bindata/manifests/cni-config/sriov/` - Template files need updates
- `api/v1/helper.go` - No changes needed (already correct)

---

## Testing Recommendation

1. **Verify NAD generation**:
   - Create SriovNetwork
   - Check NAD has resourceName in spec.config
   - Verify pciAddress in spec.config

2. **Test pod attachment**:
   - Create pod with SR-IOV network annotation
   - Verify pod becomes Ready
   - Verify network attachment succeeds

3. **Regression test**:
   - Operator restart scenarios
   - Multiple network creation
   - Pod lifecycle with networks

---

## Additional Notes

This bug only manifests when:
1. Creating NEW SriovNetwork resources (triggers NAD generation)
2. After operator restart/reinstallation
3. With comprehensive testing

Production deployments using pre-configured networks may not encounter this bug because they don't trigger new NAD generation.

---

## Attachments

Include with bug report:
1. `reproduce_incomplete_nad_bug.sh` - Automated reproduction script
2. Operator logs from `bug_evidence/` directory
3. Complete NAD output from script execution
4. `ROOT_CAUSE_AND_CODE_ANALYSIS.md` - Technical analysis
5. `BUG_EVIDENCE_AND_REPRODUCTION.md` - Proof and evidence

---

## Contact / References

**Investigation Date**: November 12, 2025  
**Investigation Status**: ✅ COMPLETE  
**Ready to File**: ✅ YES  

---

*This bug report contains all necessary information for upstream filing. Include this document along with evidence files when submitting.*

