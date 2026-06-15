(() => {
    const text = document.body ? (document.body.textContent || "") : "";
    
    // Safely count words
    const words = text.split(/\s+/).filter(Boolean);
    
    const res = {
        title: document.title,
        domNodes: document.querySelectorAll("*").length,
        images: document.querySelectorAll("img").length,
        buttons: document.querySelectorAll("button").length,
        forms: document.querySelectorAll("form").length,
        h1Count: document.querySelectorAll("h1").length,
        h2Count: document.querySelectorAll("h2").length,
        links: Array.from(document.querySelectorAll("a")).map(a => a.href).filter(Boolean),
        textLength: text.length,
        wordCount: words.length,
        hydration: {
            root: !!document.querySelector("#root"),
            next: !!document.querySelector("#__next"),
            app: !!document.querySelector("#app")
        }
    };
    
    const out = JSON.stringify(res);
    console.log("OUT::" + out);
    return out;
})();
