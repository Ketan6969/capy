(function() {
	if (typeof document === 'undefined') return;
	const proto = Object.getPrototypeOf(document);
	Object.defineProperty(proto, 'cookie', {
		get: function() {
			return typeof _goGetCookies === 'function' ? _goGetCookies() : "";
		},
		set: function(val) {
			if (typeof _goSetCookie === 'function') _goSetCookie(val);
		},
		configurable: true,
		enumerable: false
	});
})();
