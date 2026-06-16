<p align="center">
  <img src="../capy-logo.png" alt="Capy Logo" width="300" />
</p>

# Capy Scraper Guide

The Capy Runtime allows you to extract data from heavy JavaScript-rendered websites (like React, Vue, Next.js) **without** the massive CPU, RAM, and GPU overhead of running a real browser like Chromium/Playwright.

Because the engine natively parses the DOM and fully executes the website's JavaScript bundles in memory, you can write extraction scripts using standard web APIs (`document.querySelectorAll`, `getAttribute`, etc.) exactly as you would in a browser console.

---

## 1. Writing Your Extraction Script

Your script should be written in standard JavaScript. The engine will execute it *after* the website's HTML has been loaded and its JavaScript bundles have hydrated the DOM.

Create a file named `extract.js`:

```javascript
// extract.js

// 1. Target the elements you want to scrape
const articles = document.querySelectorAll('.post, .article-item');
const results = [];

// 2. Loop through the DOM nodes
for (let i = 0; i < articles.length; i++) {
    const el = articles[i];
    
    // Extract text
    const titleNode = el.querySelector('h2, .title');
    const title = titleNode ? titleNode.innerText : "No Title";
    
    // Extract attributes
    const linkNode = el.querySelector('a');
    const url = linkNode ? linkNode.getAttribute('href') : "";
    
    // Push to results array
    results.push({
        title: title.trim(),
        url: url
    });
}

// 3. The last evaluated expression in the script is returned to standard output
JSON.stringify(results, null, 2);
```

### Supported APIs
Our engine supports standard DOM traversal, including:
- `document.querySelector()` and `document.querySelectorAll()`
- `element.innerText`, `element.textContent`, `element.innerHTML`
- `element.getAttribute()`
- `element.children`, `element.parentNode`, `element.nextSibling`

---

## 2. Running the Scraper

To execute your script against a remote URL, pass the `-html` flag (the target website) and the `-file` flag (your script).

```bash
./capy -html https://news.ycombinator.com -file extract.js
```

**Output:**
```json
[
  {
    "title": "A new approach to headless browsing",
    "url": "https://example.com/article/1"
  },
  ...
]
```

---

## 3. Monitoring Performance (Memory & Speed)

To prove how lightweight the engine is compared to Playwright, you can monitor the exact execution time, Go Heap allocation, and System Memory usage by appending the `-stats` flag.

```bash
./capy -html https://reddit.com -file extract.js -stats
```

**Output Example:**
```text
[ ... JSON Output ... ]

================ CLI PERFORMANCE METRICS ================
- Execution Time: 0.406 seconds
- Go Heap Alloc:  2.85 MB
- Go System Sys:  18.77 MB
=========================================================
```

### What do the metrics mean?
- **Execution Time:** How fast the engine fetched the HTML, ran the heavy JS frameworks, built the DOM, and executed your extraction script. (Usually < 1 second).
- **Go Heap Alloc:** The actual memory allocated to parse the DOM and run the JS Virtual Machine. (Typically 2–10 MB, compared to Playwright's 150–500 MB!).
- **Go System Sys:** Total memory reserved by the Go runtime from your OS.

---

## 4. Advanced Tricks

### Wait for Dynamic Content
If a site loads data via asynchronous `fetch` calls, you can use `setTimeout` or Promises in your script, but remember the engine is fast. Often, the initial state hydration happens synchronously during the script execution phase. 

### Inline Evaluation
For quick extractions without creating a `.js` file, you can pass the JS directly using the `-eval` flag:

```bash
./capy -html https://example.com -eval "document.querySelector('h1').innerText" -stats
```
