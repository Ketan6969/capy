(function() {
			if (typeof document === 'undefined') return;
			const proto = Object.getPrototypeOf(document);
			if (!proto) return;
			
			// React interceptor: React generates random expandos like __reactContainer$vw59dcf5zx
			// We patch Math.random to pre-define these on the prototype before React assigns them!
			const origMathRandom = Math.random;
			Math.random = function() {
				const val = origMathRandom();
				const str = val.toString(36).slice(2);
				defineExpando('__reactContainer$' + str);
				defineExpando('__reactEvents$' + str);
				defineExpando('__reactFiber$' + str);
				defineExpando('__reactProps$' + str);
				defineExpando('__reactListeners$' + str);
				defineExpando('_reactListening' + str);
				return val;
			};
			// Pre-define some known static ones
			defineExpando('_reactRootContainer');
			defineExpando('__reactEvents$');
			defineExpando('__reactProps$');
			defineExpando('__reactListeners$');
			defineExpando('onclick');
			defineExpando('ondblclick');
			defineExpando('dir');
			
			var originalDollar = undefined;
			
			Object.defineProperty(proto, 'baseURI', {
				get() {
					const baseTags = document.querySelectorAll("base[href]");
					if (baseTags && baseTags.length > 0) {
						if (typeof _goResolveURL === 'function') {
							return _goResolveURL(baseTags[0].getAttribute("href"), location.href);
						}
						return baseTags[0].getAttribute("href");
					}
					return location.href;
				},
				configurable: true,
				enumerable: false
			});
			Object.defineProperty(globalThis, '$', {
				get: function() { return originalDollar !== undefined ? originalDollar : originalJQuery; },
				set: function(val) {
					originalDollar = val;
				},
				configurable: true
			});

		// Add generic polyfills for global APIs that libraries expect
			globalThis.window = globalThis;
			globalThis.HTMLIFrameElement = function() {};
			// NOTE: setTimeout, clearTimeout, setInterval, clearInterval are already
			// registered by the Go engine with proper async behavior — do NOT overwrite them here.
			if (typeof requestAnimationFrame === 'undefined') {
				globalThis.requestAnimationFrame = function(cb) { return setTimeout(cb, 16); };
			}
			if (typeof cancelAnimationFrame === 'undefined') {
				globalThis.cancelAnimationFrame = function(id) { clearTimeout(id); };
			}
			globalThis.document.readyState = 'loading';
			
			Object.defineProperty(proto, 'activeElement', {
				get: function() { return document.body || null; },
				configurable: true,
				enumerable: false
			});

			// performance API stub
			if (typeof performance === 'undefined') {
				const _perfStart = Date.now();
				globalThis.performance = {
					now: function() { return Date.now() - _perfStart; },
					mark: function() {},
					measure: function() {},
					clearMarks: function() {},
					clearMeasures: function() {},
					getEntriesByName: function() { return []; },
					getEntriesByType: function() { return []; },
					getEntries: function() { return []; },
					timing: { navigationStart: Date.now() },
					navigation: { type: 0, redirectCount: 0 },
					resourceTimingBufferSize: 150,
					setResourceTimingBufferSize: function() {},
					clearResourceTimings: function() {},
					observe: function() {}
				};
			}

			

			// screen stub
			if (typeof screen === 'undefined') {
				globalThis.screen = { width: 1920, height: 1080, availWidth: 1920, availHeight: 1080, colorDepth: 24, pixelDepth: 24 };
			}
			if (typeof devicePixelRatio === 'undefined') { globalThis.devicePixelRatio = 1; }
			if (typeof matchMedia === 'undefined') { globalThis.matchMedia = function() { return { matches: false, addListener: function(){}, removeListener: function(){}, addEventListener: function(){} }; }; }

			// Wrap Object.defineProperty to prevent Goja Host Object crashes
			const origDefineProperty = Object.defineProperty;
			const defineFallback = new WeakMap();
			Object.defineProperty = function(obj, prop, descriptor) {
				try {
					return origDefineProperty(obj, prop, descriptor);
				} catch (e) {
					const msg = e.message || '';
					// Silently absorb errors caused by Goja's host-object limitations:
					//   "cannot be made configurable" — non-configurable Go-backed property
					//   "getter must be a function"   — accessor descriptor on a host object
					//   "setter must be a function"   — same, for setters
					if (msg.includes('cannot be made configurable')) {
						if (descriptor && 'value' in descriptor) {
							try { obj[prop] = descriptor.value; } catch(err) {}
							let data = defineFallback.get(obj);
							if (!data) { data = {}; defineFallback.set(obj, data); }
							data[prop] = descriptor.value;
						}
						return obj;
					}
					if (msg.includes('getter must be a function') || msg.includes('setter must be a function')) {
						// Accessor descriptor on a host object — skip silently
						return obj;
					}
					throw e;
				}
			};

			// jQuery interceptor to support expandos on Host Objects
			var jqDataStore = {};
			var originalJQuery = undefined;
			
			function defineExpando(expando) {
				if (proto.__jqPatched && proto.__jqPatched[expando]) return;
				if (!proto.__jqPatched) {
					Object.defineProperty(proto, '__jqPatched', {
						value: {},
						writable: true,
						configurable: true,
						enumerable: false
					});
				}
				proto.__jqPatched[expando] = true;
				
				for (var i = 0; i <= 2000; i++) {
					var prop = i === 0 ? expando : expando + i;
					Object.defineProperty(proto, prop, {
						get: function() {
							const uid = this.uid;
							if (!uid) return undefined;
							return jqDataStore[uid] ? jqDataStore[uid][prop] : undefined;
						},
						set: function(v) {
							const uid = this.uid;
							if (!uid) {
								Object.defineProperty(this, prop, {value: v, writable: true, configurable: true});
								return;
							}
							if (!jqDataStore[uid]) jqDataStore[uid] = {};
							jqDataStore[uid][prop] = v;
						},
						configurable: true,
						enumerable: false
					});
				}
			}

			Object.defineProperty(globalThis, 'jQuery', {
				get: function() { return originalJQuery; },
				set: function(val) {
					originalJQuery = val;
					if (val && val.expando) {
						defineExpando(val.expando);
					}
				},
				configurable: true
			});
			
			Object.defineProperty(proto, 'implementation', {
				get() {
					if (Number(this.nodeType) === 9) { // DocumentNode
						return {
							createHTMLDocument: function() {
								const doc = {
									nodeType: 9,
									nodeName: '#document',
									childNodes: [],
									createElement: function(tag) { return document.createElement(tag); },
									createDocumentFragment: function() { return document.createDocumentFragment(); },
									body: document.createElement('body')
								};
								return doc;
							}
						};
					}
					return undefined;
				},
				configurable: true,
				enumerable: false
			});
			
			Object.defineProperty(proto, 'ownerDocument', {
				get() {
					if (Number(this.nodeType) === 9) { // DocumentNode
						return null;
					}
					return document;
				},
				configurable: true,
				enumerable: false
			});
			
			Object.defineProperty(proto, 'tagName', {
				get() {
					return this.nodeName;
				},
				configurable: true,
				enumerable: false
			});

			globalThis.Event = class Event {
				constructor(type, options) {
					this.type = type;
					this.bubbles = options ? !!options.bubbles : false;
					this.cancelable = options ? !!options.cancelable : false;
					this.defaultPrevented = false;
					this._propagationStopped = false;
					this._immediatePropagationStopped = false;
					this.target = null;
					this.currentTarget = null;
					this.eventPhase = 0;
					this.NONE = 0;
					this.CAPTURING_PHASE = 1;
					this.AT_TARGET = 2;
					this.BUBBLING_PHASE = 3;
				}
				preventDefault() { if (this.cancelable) this.defaultPrevented = true; }
				stopPropagation() { this._propagationStopped = true; }
				stopImmediatePropagation() { this._propagationStopped = true; this._immediatePropagationStopped = true; }
			};

			proto.addEventListener = function(type, callback, options) {
				const capture = typeof options === 'boolean' ? options : (options && options.capture) || false;
				if (!this.expandos) this.expandos = {};
				if (!this.expandos._listeners) this.expandos._listeners = {};
				
				const currentListeners = this.expandos._listeners[type] || [];
				this.expandos._listeners[type] = [...currentListeners, { callback, capture }];
			};

			proto.removeEventListener = function(type, callback, options) {
				const capture = typeof options === 'boolean' ? options : (options && options.capture) || false;
				if (!this.expandos || !this.expandos._listeners || !this.expandos._listeners[type]) return;
				
				const currentListeners = this.expandos._listeners[type];
				this.expandos._listeners[type] = currentListeners.filter(l => l.callback !== callback || l.capture !== capture);
			};

			proto.dispatchEvent = function(event) {
				if (!event || !event.type) return true;
				event.target = this;
				
				const path = [];
				let current = this.parentNode;
				while (current) {
					path.push(current);
					current = current.parentNode;
				}

				const dispatchPhase = (target, phase) => {
					if (event._propagationStopped) return;
					event.currentTarget = target;
					event.eventPhase = phase;
					
					if (target.expandos && target.expandos._listeners && target.expandos._listeners[event.type]) {
						const list = [...target.expandos._listeners[event.type]];
						for (const listener of list) {
							if (event._immediatePropagationStopped) break;
							
							const isCapturePhase = phase === event.CAPTURING_PHASE;
							const isTargetPhase = phase === event.AT_TARGET;
							const isBubblePhase = phase === event.BUBBLING_PHASE;
							
							if (isTargetPhase || (isCapturePhase && listener.capture) || (isBubblePhase && !listener.capture)) {
								try {
									if (typeof listener.callback === 'function') {
										listener.callback.call(target, event);
									} else if (listener.callback && typeof listener.callback.handleEvent === 'function') {
										listener.callback.handleEvent(event);
									}
								} catch (e) {
									console.error("Event listener error:", e);
								}
							}
						}
					}
				};

				for (let i = path.length - 1; i >= 0; i--) {
					dispatchPhase(path[i], event.CAPTURING_PHASE);
				}

				dispatchPhase(this, event.AT_TARGET);

				if (event.bubbles) {
					for (let i = 0; i < path.length; i++) {
						dispatchPhase(path[i], event.BUBBLING_PHASE);
					}
				}

				event.currentTarget = null;
				event.eventPhase = event.NONE;
				
				return !event.defaultPrevented;
			};

			Object.defineProperty(proto, 'innerHTML', {
				get() { 
					if (typeof this.getInnerHTML === 'function') {
						return this.getInnerHTML(); 
					}
					return undefined;
				},
				set(val) { 
					if (typeof this.setInnerHTML === 'function') {
						this.setInnerHTML(val); 
					}
				},
				configurable: true,
				enumerable: false
			});

			Object.defineProperty(proto, 'outerHTML', {
				get() { 
					if (typeof this.getOuterHTML === 'function') {
						return this.getOuterHTML(); 
					}
					return undefined;
				},
				set(val) {},
				configurable: true,
				enumerable: false
			});
			Object.defineProperty(proto, 'innerText', {
				get() { 
					if (typeof this.getInnerText === 'function') {
						return this.getInnerText(); 
					}
					return undefined;
				},
				set(val) { 
					if (typeof this.setTextContent === 'function') {
						this.setTextContent(val); 
					}
				},
				configurable: true,
				enumerable: false
			});


			Object.defineProperty(proto, 'textContent', {
				get() { 
					if (typeof this.getTextContent === 'function') {
						return this.getTextContent(); 
					}
					return undefined;
				},
				set(val) { 
					if (typeof this.setTextContent === 'function') {
						this.setTextContent(val); 
					}
				},
				configurable: true,
				enumerable: false
			});

			Object.defineProperty(proto, 'firstChild', {
				get() { 
					if (this.childNodes && this.childNodes.length > 0) {
						return this.childNodes[0];
					}
					return null;
				},
				set(val) {},
				configurable: true,
				enumerable: false
			});

			Object.defineProperty(proto, 'lastChild', {
				get() { 
					if (this.childNodes && this.childNodes.length > 0) {
						return this.childNodes[this.childNodes.length - 1];
					}
					return null;
				},
				set(val) {},
				configurable: true,
				enumerable: false
			});

			proto.remove = function() {
				if (this.parentNode) {
					this.parentNode.removeChild(this);
				}
			};

			proto.getBoundingClientRect = function() {
				return { x: 0, y: 0, width: 0, height: 0, top: 0, right: 0, bottom: 0, left: 0 };
			};

			Object.defineProperty(proto, 'classList', {
				get() {
					const node = this;
					return {
						add: function(...classes) {
							const current = (node.className || '').split(' ').filter(c => c);
							for (const cls of classes) {
								if (current.indexOf(cls) === -1) current.push(cls);
							}
							node.className = current.join(' ');
						},
						remove: function(...classes) {
							const current = (node.className || '').split(' ').filter(c => c);
							const updated = current.filter(c => classes.indexOf(c) === -1);
							node.className = updated.join(' ');
						},
						toggle: function(cls) {
							const current = (node.className || '').split(' ').filter(c => c);
							const idx = current.indexOf(cls);
							if (idx === -1) {
								current.push(cls);
								node.className = current.join(' ');
								return true;
							} else {
								current.splice(idx, 1);
								node.className = current.join(' ');
								return false;
							}
						},
						contains: function(cls) {
							const current = (node.className || '').split(' ').filter(c => c);
							return current.indexOf(cls) !== -1;
						}
					};
				},
				configurable: true,
				enumerable: false
			});

			if (typeof Intl === 'undefined') {
				globalThis.Intl = {
					DateTimeFormat: function() {
						return { format: function() { return ''; }, resolvedOptions: function() { return { locale: 'en-US' }; } };
					},
					NumberFormat: function() {
						return { format: function(n) { return n ? n.toString() : ''; }, resolvedOptions: function() { return { locale: 'en-US' }; } };
					}
				};
			}

			globalThis.Window = function Window() {};

			globalThis.__raf_count__ = 0;
			globalThis.requestAnimationFrame = function(callback) {
				if (globalThis.__raf_count__++ < 10) {
					return setTimeout(callback, 10);
				}
				return 0;
			};
			globalThis.cancelAnimationFrame = function(id) {
				clearTimeout(id);
			};

			globalThis.getComputedStyle = function(el) {
				const defaults = {
					display: 'block', visibility: 'visible', position: 'static', opacity: '1',
					color: 'rgb(0, 0, 0)', backgroundColor: 'rgba(0, 0, 0, 0)',
					width: 'auto', height: 'auto', margin: '0px', padding: '0px',
					border: '0px none rgb(0, 0, 0)', boxSizing: 'content-box',
					fontSize: '16px', fontFamily: 'sans-serif'
				};
				return new Proxy({}, {
					get: function(target, prop) {
						if (prop === 'getPropertyValue') {
							return function(p) {
								if (el && el.style && el.style[p]) return el.style[p];
								return defaults[p] || '';
							};
						}
						if (prop === 'setProperty' || prop === 'removeProperty') return function() {};
						if (el && el.style && el.style[prop]) return el.style[prop];
						return defaults[prop] || '';
					}
				});
			};

			globalThis.HTMLCanvasElement = function() {};
			globalThis.HTMLCanvasElement.prototype.getContext = function() {
				return {
					fillRect: function() {},
					clearRect: function() {},
					getImageData: function() { return { data: [] }; },
					putImageData: function() {},
					createImageData: function() { return { data: [] }; },
					setTransform: function() {},
					drawImage: function() {},
					save: function() {},
					fillText: function() {},
					restore: function() {},
					beginPath: function() {},
					moveTo: function() {},
					lineTo: function() {},
					closePath: function() {},
					stroke: function() {},
					translate: function() {},
					scale: function() {},
					rotate: function() {},
					arc: function() {},
					fill: function() {},
					measureText: function() { return { width: 0 }; },
					transform: function() {},
					rect: function() {},
					clip: function() {}
				};
			};

			globalThis.IntersectionObserver = function() {
				this.observe = function() {};
				this.unobserve = function() {};
				this.disconnect = function() {};
			};

			globalThis.ResizeObserver = function() {
				this.observe = function() {};
				this.unobserve = function() {};
				this.disconnect = function() {};
			};

			globalThis.MutationObserver = function() {
				this.observe = function() {};
				this.disconnect = function() {};
				this.takeRecords = function() { return []; };
			};
			
			globalThis.PerformanceObserver = function() {
				this.observe = function() {};
				this.disconnect = function() {};
			};

			Object.defineProperty(proto, 'href', {
				get() {
					let val = "";
					if (typeof this.getAttribute === 'function') {
						val = this.getAttribute('href') || "";
					}
					if (val && typeof document !== 'undefined' && document.baseURI && typeof _goResolveURL === 'function') {
						return _goResolveURL(val, document.baseURI);
					}
					return val;
				},
				set(val) {
					if (typeof this.setAttribute === 'function') {
						this.setAttribute('href', val);
					}
				},
				configurable: true,
				enumerable: false
			});
			
			Object.defineProperty(proto, 'src', {
				get() {
					let val = "";
					if (typeof this.getAttribute === 'function') {
						val = this.getAttribute('src') || "";
					}
					if (val && typeof document !== 'undefined' && document.baseURI && typeof _goResolveURL === 'function') {
						return _goResolveURL(val, document.baseURI);
					}
					return val;
				},
				set(val) {
					if (typeof this.setAttribute === 'function') {
						this.setAttribute('src', val);
					}
				},
				configurable: true,
				enumerable: false
			});

			globalThis.Worker = function() {
				this.postMessage = function() {};
				this.terminate = function() {};
			};
			globalThis.ServiceWorker = function() {};
			
			globalThis.crypto = {
				getRandomValues: function(arr) { return arr; },
				subtle: {
					digest: function() { return Promise.resolve(new ArrayBuffer()); },
					encrypt: function() { return Promise.resolve(new ArrayBuffer()); },
					decrypt: function() { return Promise.resolve(new ArrayBuffer()); },
					sign: function() { return Promise.resolve(new ArrayBuffer()); },
					verify: function() { return Promise.resolve(true); }
				}
			};

			globalThis.indexedDB = {
				open: function() { return { onupgradeneeded: null, onsuccess: null, onerror: null }; },
				deleteDatabase: function() { return { onsuccess: null, onerror: null }; }
			};

			globalThis.CustomEvent = function(type, params) {
				const e = new globalThis.Event(type);
				if (params) e.detail = params.detail;
				return e;
			};
			globalThis.IntersectionObserver = class IntersectionObserver {
				constructor(callback, options) {
					this.callback = callback;
				}
				observe(target) {
					if (this.callback) {
						Promise.resolve().then(() => {
							this.callback([{ target: target, isIntersecting: true, intersectionRatio: 1.0 }]);
						});
					}
				}
				unobserve(target) {}
				disconnect() {}
			};

			globalThis.ResizeObserver = class ResizeObserver {
				constructor(callback) {}
				observe(target) {}
				unobserve(target) {}
				disconnect() {}
			};

			globalThis.MutationObserver = class MutationObserver {
				constructor(callback) {}
				observe(target, options) {}
				disconnect() {}
				takeRecords() { return []; }
			};

			globalThis.history = {
				pushState: function() {},
				replaceState: function() {},
				go: function() {},
				back: function() {},
				forward: function() {},
				length: 1
			};
			globalThis.URLSearchParams = class URLSearchParams {
				constructor(init) {
					this._params = new Map();
					if (typeof init === 'string') {
						if (init.startsWith('?')) init = init.slice(1);
						const pairs = init.split('&');
						for (const p of pairs) {
							if (!p) continue;
							const idx = p.indexOf('=');
							if (idx === -1) {
								this.append(decodeURIComponent(p), '');
							} else {
								this.append(decodeURIComponent(p.slice(0, idx)), decodeURIComponent(p.slice(idx+1)));
							}
						}
					}
				}
				append(name, value) {
					if (!this._params.has(name)) this._params.set(name, []);
					this._params.get(name).push(value);
				}
				get(name) {
					const vals = this._params.get(name);
					return vals ? vals[0] : null;
				}
				getAll(name) {
					return this._params.get(name) || [];
				}
				has(name) {
					return this._params.has(name);
				}
				set(name, value) {
					this._params.set(name, [value]);
				}
				delete(name) {
					this._params.delete(name);
				}
				toString() {
					const parts = [];
					for (const [k, v] of this._params) {
						for (const val of v) {
							parts.push(encodeURIComponent(k) + '=' + encodeURIComponent(val));
						}
					}
					return parts.join('&');
				}
			};

			Object.defineProperty(proto, 'body', {
				get() {
					if (Number(this.nodeType) === 9) { // DocumentNode
						if (typeof this.getBody === 'function') {
							return this.getBody();
						}
					}
					return undefined;
				},
				set(val) {},
				configurable: true,
				enumerable: false
			});

			Object.defineProperty(proto, 'head', {
				get() {
					if (Number(this.nodeType) === 9) { // DocumentNode
						if (typeof this.getHead === 'function') {
							return this.getHead();
						}
					}
					return undefined;
				},
				set(val) {},
				configurable: true,
				enumerable: false
			});

			Object.defineProperty(proto, 'documentElement', {
				get() {
					if (Number(this.nodeType) === 9) { // DocumentNode
						if (typeof this.getDocumentElement === 'function') {
							return this.getDocumentElement();
						}
					}
					return undefined;
				},
				set(val) {},
				configurable: true,
				enumerable: false
			});

			proto.write = function(markup) {
				const body = document.body || document;
				const temp = document.createElement('div');
				temp.innerHTML = markup;
				while (temp.childNodes && temp.childNodes.length > 0) {
					body.appendChild(temp.childNodes[0]);
				}
			};

			// Bind length getter to the Storage prototype
			if (typeof localStorage !== 'undefined') {
				const storageProto = Object.getPrototypeOf(localStorage);
				if (storageProto) {
					Object.defineProperty(storageProto, 'length', {
						get() { 
							if (typeof this.getLength === 'function') {
								return this.getLength(); 
							}
							return 0;
						},
						configurable: true,
						enumerable: false
					});
				}
			}
		})();
