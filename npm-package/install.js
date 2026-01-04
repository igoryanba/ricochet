#!/usr/bin/env node

const https = require('https');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const REPO = 'igoryanba/ricochet';
const BIN_DIR = path.join(__dirname, 'bin');
const BIN_NAME = process.platform === 'win32' ? 'ricochet.exe' : 'ricochet';
const BIN_PATH = path.join(BIN_DIR, BIN_NAME);

// Determine platform and architecture
function getPlatformInfo() {
    const platform = process.platform;
    const arch = process.arch;

    // Map to GitHub release asset names
    const platformMap = {
        darwin: 'darwin',
        linux: 'linux',
        win32: 'windows'
    };

    const archMap = {
        x64: 'amd64',
        arm64: 'arm64',
        arm: 'arm'
    };

    const osPart = platformMap[platform];
    const archPart = archMap[arch];

    if (!osPart || !archPart) {
        throw new Error(`Unsupported platform: ${platform}-${arch}`);
    }

    const ext = platform === 'win32' ? '.exe' : '';
    return `ricochet-${osPart}-${archPart}${ext}`;
}

// Get latest release download URL
async function getLatestReleaseUrl(assetName) {
    return new Promise((resolve, reject) => {
        const options = {
            hostname: 'api.github.com',
            path: `/repos/${REPO}/releases/latest`,
            headers: {
                'User-Agent': 'ricochet-mcp-installer'
            }
        };

        https.get(options, (res) => {
            let data = '';
            res.on('data', chunk => data += chunk);
            res.on('end', () => {
                try {
                    const release = JSON.parse(data);
                    const asset = release.assets.find(a => a.name === assetName);
                    if (!asset) {
                        reject(new Error(`Asset ${assetName} not found in release ${release.tag_name}`));
                        return;
                    }
                    resolve(asset.browser_download_url);
                } catch (e) {
                    reject(e);
                }
            });
        }).on('error', reject);
    });
}

// Download file with redirect support
function downloadFile(url, dest) {
    return new Promise((resolve, reject) => {
        const file = fs.createWriteStream(dest);

        const request = (url) => {
            https.get(url, (res) => {
                // Handle redirect
                if (res.statusCode === 302 || res.statusCode === 301) {
                    request(res.headers.location);
                    return;
                }

                if (res.statusCode !== 200) {
                    reject(new Error(`Failed to download: ${res.statusCode}`));
                    return;
                }

                res.pipe(file);
                file.on('finish', () => {
                    file.close();
                    resolve();
                });
            }).on('error', (err) => {
                fs.unlink(dest, () => { });
                reject(err);
            });
        };

        request(url);
    });
}

async function main() {
    console.log('🚀 Installing Ricochet MCP...');

    // Create bin directory
    if (!fs.existsSync(BIN_DIR)) {
        fs.mkdirSync(BIN_DIR, { recursive: true });
    }

    // Skip if binary already exists
    if (fs.existsSync(BIN_PATH)) {
        console.log('✅ Ricochet binary already installed');
        return;
    }

    try {
        const assetName = getPlatformInfo();
        console.log(`📦 Downloading ${assetName}...`);

        const url = await getLatestReleaseUrl(assetName);
        console.log(`⬇️  Downloading from GitHub releases...`);

        await downloadFile(url, BIN_PATH);

        // Make executable on Unix
        if (process.platform !== 'win32') {
            fs.chmodSync(BIN_PATH, 0o755);
        }

        console.log('✅ Ricochet installed successfully!');
        console.log('');
        console.log('📱 Next steps:');
        console.log('   1. Create a Telegram bot via @BotFather');
        console.log('   2. Set TELEGRAM_BOT_TOKEN environment variable');
        console.log('   3. Restart your IDE');
        console.log('');
    } catch (error) {
        console.error('❌ Installation failed:', error.message);
        console.error('');
        console.error('Manual installation:');
        console.error('  curl -L https://github.com/igoryanba/ricochet/releases/latest/download/' + getPlatformInfo() + ' -o ricochet');
        console.error('  chmod +x ricochet');
        process.exit(1);
    }
}

main();
