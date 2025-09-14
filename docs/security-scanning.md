# Security Scanning Documentation

## Overview

CryptoPulse implements a comprehensive security scanning strategy that differentiates between production dependencies and test-only dependencies to provide accurate security assessments.

## Scanning Strategy

### Two-Tier Scanning Approach

#### 1. Production Dependencies Scan (Critical)
- **Purpose**: Identifies vulnerabilities in dependencies that are included in production builds
- **Scope**: Main application dependencies only
- **Action**: Fails pipeline if critical vulnerabilities found
- **Configuration**: Uses `.trivyignore` to exclude test-only dependencies

#### 2. All Dependencies Scan (Informational)
- **Purpose**: Provides complete visibility into all dependencies including test libraries
- **Scope**: All dependencies (production + test + development)
- **Action**: Informational only, does not fail pipeline
- **Configuration**: No exclusions, scans everything

## Ignored Vulnerabilities

### Test-Only Dependencies

The following vulnerabilities are ignored because they affect test-only dependencies that are not included in production builds:

#### CVE-2025-30153
- **Package**: `github.com/getkin/kin-openapi`
- **Severity**: HIGH
- **Issue**: Improper Handling of Highly Compressed Data (Data Amplification)
- **Reason for Ignoring**: 
  - Only used in `tests/api/` for OpenAPI contract validation
  - Not included in production binary (`./cmd/main.go`)
  - Not included in Docker images (production builds use `CGO_ENABLED=0`)
  - Test environment is controlled and not exposed to external data

### Verification Process

To verify that ignored dependencies are truly test-only:

1. **Check Import Usage**:
   ```bash
   # Search for imports in production code
   grep -r "github.com/getkin/kin-openapi" pkg/ cmd/
   # Should return no results
   
   # Search for imports in test code
   grep -r "github.com/getkin/kin-openapi" tests/
   # Should show usage only in tests/api/
   ```

2. **Check Binary Dependencies**:
   ```bash
   # Build production binary
   go build -o cryptopulse ./cmd/main.go
   
   # Check binary dependencies (if available)
   go version -m cryptopulse | grep kin-openapi
   # Should return no results
   ```

3. **Check Docker Image**:
   ```bash
   # Build production Docker image
   docker build -f Dockerfile.prod -t cryptopulse:security-check .
   
   # Check for test dependencies in image
   docker run --rm cryptopulse:security-check find / -name "*kin-openapi*" 2>/dev/null
   # Should return no results
   ```

## Security Scan Configuration

### Trivy Configuration

#### Production Scan
```yaml
- name: Run Trivy vulnerability scanner (Production Dependencies)
  uses: aquasecurity/trivy-action@master
  with:
    scan-type: 'fs'
    scan-ref: '.'
    format: 'sarif'
    output: 'trivy-results.sarif'
    trivyignores: '.trivyignore'
```

#### All Dependencies Scan
```yaml
- name: Run Trivy vulnerability scanner (All Dependencies - Informational)
  uses: aquasecurity/trivy-action@master
  with:
    scan-type: 'fs'
    scan-ref: '.'
    format: 'table'
    output: 'trivy-all-results.txt'
  continue-on-error: true
```

### Ignore File Format

The `.trivyignore` file uses the following format:
```
# Comment explaining the vulnerability
CVE-YYYY-NNNNN

# Example:
# CVE-2025-30153: Data Amplification in kin-openapi
# Only used in tests/api/ for contract validation
CVE-2025-30153
```

## Monitoring and Maintenance

### Regular Review Process

1. **Monthly Review**: Review all ignored vulnerabilities
2. **Dependency Updates**: Check if fixed versions are available
3. **Usage Verification**: Confirm dependencies are still test-only
4. **Documentation Updates**: Update this document with any changes

### Adding New Ignores

When adding new vulnerabilities to ignore:

1. **Verify Test-Only Usage**:
   ```bash
   # Check where the dependency is used
   grep -r "package-name" pkg/ cmd/ tests/
   ```

2. **Document the Reason**:
   - Add comment to `.trivyignore`
   - Update this documentation
   - Include verification steps

3. **Set Review Date**:
   - Add calendar reminder to review in 3 months
   - Check for updates to the vulnerable package

### Escalation Process

If a vulnerability is found in a production dependency:

1. **Immediate Assessment**: Determine impact and exploitability
2. **Update Priority**: 
   - Critical/High: Update within 24-48 hours
   - Medium: Update within 1 week
   - Low: Update in next maintenance cycle
3. **Testing**: Run full test suite after updates
4. **Deployment**: Deploy security updates as hotfixes if critical

## Security Scan Results

### Understanding Results

#### Production Scan Results
- **0 vulnerabilities**: ✅ Safe for production deployment
- **Low/Medium vulnerabilities**: ⚠️ Review and plan updates
- **High/Critical vulnerabilities**: ❌ Block deployment, immediate fix required

#### All Dependencies Scan Results
- **Test-only vulnerabilities**: ℹ️ Informational, verify they're truly test-only
- **New production vulnerabilities**: 🚨 Investigate and potentially add to production scan

### Artifacts Generated

1. **trivy-results.sarif**: Production dependencies scan (uploaded to GitHub Security)
2. **trivy-all-results.txt**: Complete dependency scan (informational)
3. **.trivyignore**: List of ignored vulnerabilities with reasons

## Best Practices

### Dependency Management

1. **Separate Test Dependencies**: Keep test dependencies in separate modules when possible
2. **Minimal Production Dependencies**: Only include necessary dependencies in production builds
3. **Regular Updates**: Update dependencies regularly, especially security patches
4. **Vulnerability Monitoring**: Subscribe to security advisories for key dependencies

### Build Security

1. **Multi-stage Builds**: Use multi-stage Docker builds to exclude test dependencies
2. **Static Builds**: Use `CGO_ENABLED=0` for static binaries without external dependencies
3. **Minimal Base Images**: Use minimal base images (alpine, scratch) for production
4. **Dependency Verification**: Verify checksums and signatures when possible

## Troubleshooting

### Common Issues

1. **False Positives**: Vulnerability reported in test-only dependency
   - **Solution**: Verify usage, add to `.trivyignore` with documentation

2. **Missing Vulnerabilities**: Known vulnerability not detected
   - **Solution**: Update Trivy database, check scan configuration

3. **Scan Failures**: Security scan job fails
   - **Solution**: Check Trivy action version, network connectivity, file permissions

### Support Resources

- [Trivy Documentation](https://aquasecurity.github.io/trivy/)
- [GitHub Security Advisories](https://github.com/advisories)
- [Go Security Database](https://pkg.go.dev/vuln/)
- [CVE Database](https://cve.mitre.org/)
