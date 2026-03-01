# Deployment Guide - LCC.live iOS App

## Overview

This guide covers deploying the LCC.live iOS app to TestFlight and the App Store.

## Prerequisites

### Required Accounts
- Apple Developer Account ($99/year)
- App Store Connect access
- GitHub account (for CI/CD)

### Required Tools
- Xcode 15.2 or later
- Fastlane (installed via `gem install fastlane`)
- Ruby 3.2+ (for Fastlane)

## Initial Setup

### 1. Apple Developer Setup

1. **Create App ID**
   - Go to [Apple Developer Portal](https://developer.apple.com)
   - Certificates, Identifiers & Profiles → Identifiers
   - Create App ID: `live.lcc`
   - Enable capabilities: None required (add as needed)

2. **Create App Store Connect Record**
   - Go to [App Store Connect](https://appstoreconnect.apple.com)
   - My Apps → + → New App
   - Bundle ID: `live.lcc`
   - Name: "LCC.live"
   - Primary Language: English
   - SKU: `lcc-live-ios`

3. **Generate App Store Connect API Key**
   - App Store Connect → Users and Access → Keys
   - Generate new API Key (role: App Manager)
   - Download `.p8` file
   - Note: Key ID and Issuer ID

### 2. Code Signing Setup

#### Option A: Manual (Recommended for first time)

1. **Create Certificates**
   ```bash
   # Development certificate
   open "https://developer.apple.com/account/resources/certificates/add"
   # Select "Apple Development"
   
   # Distribution certificate
   # Select "Apple Distribution"
   ```

2. **Create Provisioning Profiles**
   ```bash
   # Development profile
   open "https://developer.apple.com/account/resources/profiles/add"
   # Select iOS App Development
   # Choose App ID: live.lcc
   # Select certificates and devices
   
   # App Store profile
   # Select App Store
   # Choose App ID: live.lcc
   # Select distribution certificate
   ```

3. **Download and Install**
   - Download certificates and profiles
   - Double-click to install in Xcode

#### Option B: Fastlane Match (Recommended for teams)

1. **Create Private Git Repository**
   ```bash
   # On GitHub, create a private repo: ios-certificates
   ```

2. **Initialize Match**
   ```bash
   fastlane match init
   # Choose git storage
   # Enter repo URL: git@github.com:your-username/ios-certificates.git
   ```

3. **Generate Certificates**
   ```bash
   # Development
   fastlane match development
   
   # App Store
   fastlane match appstore
   ```

### 3. GitHub Secrets Setup

Add the following secrets in GitHub repo settings (Settings → Secrets and variables → Actions):

#### Required Secrets

```bash
# Apple Account
APPLE_ID=your.email@example.com
TEAM_ID=T93925EZUT  # Your Apple Developer Team ID

# App Store Connect
ITC_TEAM_ID=your_itc_team_id
APP_STORE_CONNECT_API_KEY_ID=ABC123XYZ
APP_STORE_CONNECT_API_ISSUER_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
APP_STORE_CONNECT_API_KEY=<base64 encoded .p8 file>

# Code Signing
CERTIFICATES_P12=<base64 encoded .p12 certificate>
CERTIFICATES_P12_PASSWORD=your_cert_password
PROVISIONING_PROFILE=<base64 encoded .mobileprovision>

# Fastlane Match (if using)
MATCH_PASSWORD=your_match_encryption_password
MATCH_GIT_URL=git@github.com:your-username/ios-certificates.git

# App Specific Password (for Fastlane)
APP_SPECIFIC_PASSWORD=xxxx-xxxx-xxxx-xxxx

# Optional: Backend Configuration
LCC_API_BASE_URL=https://lcc.live
GRAFANA_METRICS_URL=https://lcc.live/api/metrics
```

#### Encoding Files for Secrets

```bash
# Encode certificate (.p12)
base64 -i Certificates.p12 | pbcopy

# Encode provisioning profile
base64 -i profile.mobileprovision | pbcopy

# Encode API key (.p8)
base64 -i AuthKey_ABC123XYZ.p8 | pbcopy
```

## Deployment Workflows

### Local Development Build

```bash
# Setup environment
./scripts/setup_env.sh

# Run tests
fastlane test

# Build locally
fastlane build

# Upload to TestFlight
fastlane beta
```

### Automated CI/CD (GitHub Actions)

1. **Push to main branch** triggers automatic:
   - Tests
   - Build
   - TestFlight upload

2. **Monitor deployment**
   ```bash
   # Check GitHub Actions
   open "https://github.com/your-username/lcc.live.ios/actions"
   ```

3. **Builds appear in TestFlight within ~10 minutes**

### Manual TestFlight Upload

```bash
# Increment version
fastlane bump_version type:patch  # or minor, major

# Build and upload
fastlane build
fastlane beta
```

## TestFlight Beta Testing

### Internal Testing

1. **Add Internal Testers**
   - App Store Connect → TestFlight → Internal Testing
   - Add up to 100 internal testers (no review required)

2. **Distribute Build**
   - Select build
   - Add to internal group
   - Testers receive email immediately

### External Testing

1. **Submit for Beta App Review**
   - App Store Connect → TestFlight → External Testing
   - Create external group
   - Add test information
   - Submit for review (usually 24-48 hours)

2. **Add External Testers**
   - Up to 10,000 external testers
   - Send invitation link
   - Testers install TestFlight app

## App Store Release

### 1. Prepare App Store Listing

```bash
# Generate screenshots
fastlane screenshots
```

Fill in App Store Connect:
- App description
- Keywords
- Screenshots (all required sizes)
- App icon (1024x1024)
- Privacy policy URL
- Support URL

### 2. Submit for Review

```bash
# Release build
fastlane release
```

Or manually in App Store Connect:
1. Select build for release
2. Fill in "What's New" section
3. Submit for review

### 3. Review Process

- Initial review: 24-48 hours typically
- Respond to reviewer feedback promptly
- Common rejection reasons:
  - Missing privacy policy
  - Crashes on launch
  - Incomplete functionality

## Version Management

### Semantic Versioning

We use semantic versioning: `MAJOR.MINOR.PATCH (BUILD)`

```bash
# Patch release (bug fixes): 1.0.0 → 1.0.1
fastlane bump_version type:patch

# Minor release (new features): 1.0.1 → 1.1.0
fastlane bump_version type:minor

# Major release (breaking changes): 1.1.0 → 2.0.0
fastlane bump_version type:major
```

### Build Numbers

- Automatically incremented by CI/CD
- Manual increment:
  ```bash
  ./scripts/increment_build.sh
  ```

## Monitoring & Metrics

### TestFlight Analytics

- App Store Connect → TestFlight → App Analytics
- Monitor: installs, sessions, crashes

### Crash Reporting

1. **Xcode Organizer**
   - Window → Organizer → Crashes
   - Download crash logs
   - Symbolicate and analyze

2. **App Store Connect**
   - App Analytics → Crashes
   - Real-time crash data

### Custom Metrics (Grafana)

- Metrics automatically sent to `https://lcc.live/api/metrics`
- View in Grafana dashboards
- Track: API performance, image load times, user engagement

## Troubleshooting

### Build Failures

```bash
# Clean build
xcodebuild clean

# Reset derived data
rm -rf ~/Library/Developer/Xcode/DerivedData

# Update certificates
fastlane match appstore --force
```

### Signing Issues

```bash
# Verify certificate
security find-identity -v -p codesigning

# Check provisioning profile
fastlane sigh
```

### TestFlight Upload Fails

```bash
# Validate build
fastlane pilot validate

# Check API key permissions
# Ensure key has "App Manager" role in App Store Connect
```

### CI/CD Failures

1. Check GitHub Actions logs
2. Verify all secrets are set correctly
3. Ensure Xcode version matches in workflow
4. Check certificate expiration dates

## Maintenance

### Regular Tasks

- **Monthly**: Review TestFlight feedback
- **Quarterly**: Update dependencies, Xcode version
- **Yearly**: Renew certificates (auto with Match)

### Certificate Renewal

Certificates expire after 1 year. With Fastlane Match:

```bash
fastlane match appstore --force
```

Without Match:
1. Revoke old certificate in Developer Portal
2. Create new certificate
3. Update GitHub secrets
4. Regenerate provisioning profiles

## Support

### Resources

- [Apple Developer Documentation](https://developer.apple.com/documentation/)
- [Fastlane Documentation](https://docs.fastlane.tools/)
- [TestFlight Guide](https://developer.apple.com/testflight/)

### Getting Help

- Check TROUBLESHOOTING.md
- Review GitHub Actions logs
- Check Fastlane output

## Checklist

### Pre-Launch Checklist

- [ ] All tests passing
- [ ] App Store screenshots ready
- [ ] App description written
- [ ] Privacy policy published
- [ ] Support email/website set up
- [ ] TestFlight beta testing completed
- [ ] No critical bugs
- [ ] Performance tested on all devices
- [ ] App Store review guidelines verified

### Post-Launch Checklist

- [ ] Monitor crash reports
- [ ] Respond to user reviews
- [ ] Track metrics in Grafana
- [ ] Plan next release

