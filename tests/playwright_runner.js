const { chromium } = require('playwright');
const fs = require('fs');

(async () => {
    const url = process.argv[2];
    const scriptPath = process.argv[3];
    
    if (!url || !scriptPath) {
        console.error("Usage: node playwright_runner.js <url> <scriptPath>");
        process.exit(1);
    }
    
    const browser = await chromium.launch({ headless: true });
    const page = await browser.newPage();
    
    let reqCount = 0;
    page.on('request', () => { reqCount++; });
    
    try {
        await page.goto(url, { waitUntil: 'networkidle', timeout: 15000 });
    } catch(e) {
        // Ignore timeout, we will extract whatever is there
    }
    
    try {
        const scriptContent = fs.readFileSync(scriptPath, 'utf8');
        const result = await page.evaluate(scriptContent);
        console.log("OUT::" + result);
        console.log("NET_REQ::" + reqCount);
    } catch(e) {
        console.error(e);
    }
    
    await browser.close();
})();
