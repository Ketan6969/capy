const res = {
	title: document.title,
	h1: document.querySelector('h1') ? document.querySelector('h1').innerText.trim().replace(/\n/g, ' ') : null,
	links: document.querySelectorAll('a').length
};
const out = JSON.stringify(res);
console.log("OUT::" + out);
out;
