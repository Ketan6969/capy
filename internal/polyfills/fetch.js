(function() {
			class Response {
				constructor(rawRes) {
					this.status = rawRes.status;
					this.statusText = rawRes.statusText;
					this.ok = this.status >= 200 && this.status < 300;
					this._bodyText = rawRes.body;
					this.headers = new Map(Object.entries(rawRes.headers || {}));
				}

				async text() {
					return this._bodyText;
				}

				async json() {
					return JSON.parse(this._bodyText);
				}
			}

			globalThis.fetch = function(url, options) {
				return new Promise((resolve, reject) => {
					_goFetch(url, options || {}, (err, rawRes) => {
						if (err) {
							reject(new Error(err));
						} else {
							resolve(new Response(rawRes));
						}
					});
				});
			};
		})();
