const startTime = Date.now();
const results = {
    pageLoaded: false,
    jsExecuted: false,
    domUpdated: false,
    fetchWorked: false,
    timersWorked: false,
    eventsWorked: false,
    storageWorked: false
};

// 1. Page Loaded
const links = document.querySelectorAll("link");
let foundPreload = false;
for (let i = 0; i < links.length; i++) {
    if (links[i].getAttribute("as") === "fetch") {
        foundPreload = true;
    }
}
if (typeof document !== "undefined" && document.body && foundPreload) {
    results.pageLoaded = true;
}

// 2. JS Executed
results.jsExecuted = true;

// 3. DOM Updated
const mapsRoot = document.getElementById("maps-root");
if (mapsRoot) {
    const el = document.createElement("div");
    el.id = "maps-appended";
    el.textContent = "Google Maps View rendered";
    mapsRoot.appendChild(el);
    const checkEl = document.getElementById("maps-appended");
    if (checkEl && checkEl.textContent === "Google Maps View rendered") {
        results.domUpdated = true;
    }
}

// 4. Storage Worked
if (typeof localStorage !== "undefined") {
    localStorage.setItem("test-key", "test-val");
    if (localStorage.getItem("test-key") === "test-val") {
        results.storageWorked = true;
    }
}

// 5. Events Worked
let eventReceived = false;
document.addEventListener("bench-click", () => {
    eventReceived = true;
});
const ev = new Event("bench-click");
document.dispatchEvent(ev);
if (eventReceived) {
    results.eventsWorked = true;
}

// 6. Timers worked
let timerFired = false;
setTimeout(() => {
    timerFired = true;
    checkCompletion();
}, 20);

// 7. Fetch worked
let fetchFired = false;
fetch("https://jsonplaceholder.typicode.com/posts/1")
    .then(res => res.json())
    .then(data => {
        if (data && data.id === 1) {
            fetchFired = true;
        }
        checkCompletion();
    })
    .catch(err => {
        console.error("Fetch error:", err);
        checkCompletion();
    });

function checkCompletion() {
    if (timerFired && fetchFired) {
        results.timersWorked = true;
        results.fetchWorked = true;
        printResults();
    }
}

// Timeout fallback
setTimeout(() => {
    results.timersWorked = timerFired;
    results.fetchWorked = fetchFired;
    printResults();
}, 2000);

let resultsPrinted = false;
function printResults() {
    if (resultsPrinted) return;
    resultsPrinted = true;
    console.log(`[RESULT] Page Loaded: ${results.pageLoaded ? "PASS" : "FAIL"}`);
    console.log(`[RESULT] JS Executed: ${results.jsExecuted ? "PASS" : "FAIL"}`);
    console.log(`[RESULT] DOM Updated: ${results.domUpdated ? "PASS" : "FAIL"}`);
    console.log(`[RESULT] Fetch Worked: ${results.fetchWorked ? "PASS" : "FAIL"}`);
    console.log(`[RESULT] Timers Worked: ${results.timersWorked ? "PASS" : "FAIL"}`);
    console.log(`[RESULT] Events Worked: ${results.eventsWorked ? "PASS" : "FAIL"}`);
    console.log(`[RESULT] Storage Worked: ${results.storageWorked ? "PASS" : "FAIL"}`);
}
