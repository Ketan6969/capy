// CONFIGURATION: Adjust this number to scrape any number of places (maximum 100 per single API query)
const SCRAPE_LIMIT = 500;
const startTime = Date.now();

console.log("Analyzing Google Maps page...");

// Find the preloaded fetch link by querying all links and filtering by attribute
const links = document.querySelectorAll("link");
let preloadLink = null;
for (let i = 0; i < links.length; i++) {
    if (links[i].getAttribute("as") === "fetch") {
        preloadLink = links[i];
        break;
    }
}

if (!preloadLink) {
    console.error("Error: Preloaded fetch link not found in DOM!");
} else {
    let href = preloadLink.getAttribute("href");
    console.log("Found API relative path:", href.substring(0, 100) + "...");

    // Extract search query
    const queryMatch = href.match(/[?&]q=([^&]+)/);
    const query = queryMatch ? decodeURIComponent(queryMatch[1].replace(/\+/g, ' ')) : "search query";

    // Request items matching SCRAPE_LIMIT (up to 100 per page)
    const apiLimit = Math.min(SCRAPE_LIMIT, 100);
    href = href.replace("%217i20", "%217i" + apiLimit).replace("!7i20", "!7i" + apiLimit);

    // Normalize to absolute URL
    if (href.startsWith("/")) {
        href = "https://www.google.com" + href;
    }

    console.log("Fetching live Google Maps API data...");
    fetch(href)
        .then(res => res.text())
        .then(text => {
            console.log("Response text length:", text.length);

            // Google Map JSON responses are prefixed with ")]}'\n" for security
            const securityPrefix = ")]}'\n";
            let cleanJSON = text;
            if (text.startsWith(securityPrefix)) {
                cleanJSON = text.substring(securityPrefix.length);
            } else {
                const firstNL = text.indexOf("\n");
                if (firstNL !== -1 && firstNL < 10) {
                    cleanJSON = text.substring(firstNL + 1);
                }
            }

            const data = JSON.parse(cleanJSON);
            console.log("Successfully parsed API JSON structure.");
            console.log("Traversing nested array...");

            const listings = [];

            function findListings(arr) {
                if (!Array.isArray(arr)) return;

                // Identify single business listing arrays (typically length > 20 and index 14 is the name)
                if (arr.length > 20 && typeof arr[14] === "string" && arr[14].length > 1) {
                    const name = arr[14];
                    const rating = arr[7] ? arr[7][0] : null;
                    const reviews = arr[7] ? arr[7][1] : null;

                    let address = "";
                    if (Array.isArray(arr[2])) {
                        address = arr[2].join(", ");
                    } else if (typeof arr[18] === "string") {
                        address = arr[18];
                    } else if (Array.isArray(arr[18]) && typeof arr[18][0] === "string") {
                        address = arr[18][0];
                    }

                    let category = "";
                    if (Array.isArray(arr[13])) {
                        category = arr[13].join(", ");
                    }

                    listings.push({
                        name: name,
                        rating: rating,
                        reviews: reviews,
                        address: address,
                        category: category
                    });
                    return;
                }

                for (let i = 0; i < arr.length; i++) {
                    if (Array.isArray(arr[i])) {
                        findListings(arr[i]);
                    }
                }
            }

            findListings(data);

            console.log(`\n================ FOUND ${listings.length} LISTINGS FOR: ${query.toUpperCase()} ================`);
            listings.slice(0, SCRAPE_LIMIT).forEach((cafe, idx) => {
                console.log(`\n[Place #${idx + 1}]`);
                console.log(`- Name:     ${cafe.name}`);
                console.log(`- Rating:   ${cafe.rating} (${cafe.reviews} reviews)`);
                console.log(`- Category: ${cafe.category || 'Cafe'}`);
                if (cafe.address) {
                    console.log(`- Address:  ${cafe.address}`);
                }
            });
            console.log("\n==================================================================");

            // Print performance metrics
            const duration = (Date.now() - startTime) / 1000;
            console.log(`\n================ PERFORMANCE METRICS ================`);
            console.log(`- Execution Time: ${duration.toFixed(3)} seconds`);
            
            if (typeof _goGetMemoryUsage === "function") {
                const m = _goGetMemoryUsage();
                const allocMB = (m.alloc / (1024 * 1024)).toFixed(2);
                const sysMB = (m.sys / (1024 * 1024)).toFixed(2);
                console.log(`- Go Heap Alloc:  ${allocMB} MB`);
                console.log(`- Go System Sys:  ${sysMB} MB`);
            }
            console.log(`======================================================`);
        })
        .catch(err => {
            console.error("Error fetching or parsing Maps data:", err);
            const duration = (Date.now() - startTime) / 1000;
            console.log(`\n================ PERFORMANCE METRICS ================`);
            console.log(`- Execution Time: ${duration.toFixed(3)} seconds (Failed)`);
            console.log(`======================================================`);
        });
}
