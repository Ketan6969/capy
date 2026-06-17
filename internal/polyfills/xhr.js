// XMLHttpRequest polyfill backed by fetch
if (typeof XMLHttpRequest === 'undefined') {
	globalThis.XMLHttpRequest = function() {
		this.readyState = 0;
		this.status = 0;
		this.statusText = '';
		this.responseText = '';
		this.responseXML = null;
		this.onreadystatechange = null;
		this.onload = null;
		this.onerror = null;
		this.responseType = '';
		this.response = '';
		this._method = 'GET';
		this._url = '';
		this._headers = {};
		this._resHeaders = null;
	};
	globalThis.XMLHttpRequest.prototype.open = function(method, url) {
		this._method = method;
		this._url = url;
		this.readyState = 1;
		if (this.onreadystatechange) this.onreadystatechange();
	};
	globalThis.XMLHttpRequest.prototype.send = function(body) {
		this.readyState = 2;
		if (this.onreadystatechange) this.onreadystatechange();

		const self = this;
		const options = { method: this._method, headers: this._headers };
		if (body) options.body = body;

		fetch(this._url, options).then(res => {
			self.status = res.status;
			self.statusText = res.statusText;
			self._resHeaders = res.headers;
			self.readyState = 3;
			if (self.onreadystatechange) self.onreadystatechange();
			return res.text();
		}).then(text => {
			self.responseText = text;
			self.response = text;
			self.readyState = 4;
			if (self.onreadystatechange) self.onreadystatechange();
			if (self.onload) self.onload();
		}).catch(err => {
			self.readyState = 4;
			if (self.onreadystatechange) self.onreadystatechange();
			if (self.onerror) self.onerror(err);
		});
	};
	globalThis.XMLHttpRequest.prototype.setRequestHeader = function(k, v) {
		this._headers[k] = v;
	};
	globalThis.XMLHttpRequest.prototype.getResponseHeader = function(name) {
		if (this.readyState < 2 || !this._resHeaders) {
			return null;
		}
		// Fetch headers map is typically lowercased
		let lowerName = name.toLowerCase();
		if (this._resHeaders.has(lowerName)) {
			return this._resHeaders.get(lowerName);
		}
		// fallback to check original case if fetch implementation doesn't lowercase
		for (let [k, v] of this._resHeaders.entries()) {
			if (k.toLowerCase() === lowerName) {
				return v;
			}
		}
		return null;
	};
	globalThis.XMLHttpRequest.prototype.getAllResponseHeaders = function() {
		if (this.readyState < 2 || !this._resHeaders) {
			return null;
		}
		let result = '';
		this._resHeaders.forEach((value, key) => {
			result += key + ': ' + value + '\r\n';
		});
		return result;
	};
	globalThis.XMLHttpRequest.prototype.abort = function() {};
	globalThis.XMLHttpRequest.prototype.addEventListener = function(evt, cb) {
		if (evt === 'load') this.onload = cb;
		if (evt === 'error') this.onerror = cb;
		if (evt === 'readystatechange') this.onreadystatechange = cb;
	};
	globalThis.XMLHttpRequest.UNSENT = 0;
	globalThis.XMLHttpRequest.OPENED = 1;
	globalThis.XMLHttpRequest.HEADERS_RECEIVED = 2;
	globalThis.XMLHttpRequest.LOADING = 3;
	globalThis.XMLHttpRequest.DONE = 4;
}
