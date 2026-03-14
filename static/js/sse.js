(function() {
    "use strict";

    var SSE_SOURCES = {};
    var pageUnloading = false;

    window.addEventListener("beforeunload", function() {
        pageUnloading = true;
    });

    htmx.defineExtension("sse", {
        onEvent: function(name, evt) {
            if (name === "htmx:load") {
                var elt = evt.detail.elt;
                var connectUrl = elt.getAttribute("sse-connect");
                if (!connectUrl) {
                    return;
                }
                setupSSE(elt, connectUrl);
            }

            if (name === "htmx:beforeCleanupElement") {
                var elt2 = evt.detail.elt;
                var sourceId = elt2.getAttribute("data-sse-id");
                if (sourceId && SSE_SOURCES[sourceId]) {
                    SSE_SOURCES[sourceId].source.close();
                    if (SSE_SOURCES[sourceId].reconnectTimer) {
                        clearTimeout(SSE_SOURCES[sourceId].reconnectTimer);
                    }
                    delete SSE_SOURCES[sourceId];
                }
            }
        }
    });

    function setupSSE(elt, url) {
        var id = "sse-" + Date.now() + "-" + Math.random().toString(36).substring(2, 11);
        elt.setAttribute("data-sse-id", id);

        var backoff = 1000;
        var maxBackoff = 128000;

        function connect() {
            var source = new EventSource(url, { withCredentials: true });

            SSE_SOURCES[id] = {
                source: source,
                reconnectTimer: null
            };

            source.onopen = function() {
                backoff = 1000;
            };

            source.onerror = function() {
                if (pageUnloading) {
                    return;
                }
                source.close();
                htmx.trigger(elt, "htmx:sseError");
                SSE_SOURCES[id].reconnectTimer = setTimeout(function() {
                    if (SSE_SOURCES[id]) {
                        connect();
                    }
                }, backoff);
                backoff = Math.min(backoff * 2, maxBackoff);
            };

            // Find all sse-swap targets within this element
            var swapTargets = elt.querySelectorAll("[sse-swap]");
            swapTargets.forEach(function(target) {
                var eventName = target.getAttribute("sse-swap");
                source.addEventListener(eventName, function(e) {
                    target.innerHTML = e.data;
                    htmx.process(target);
                });
            });
        }

        connect();
    }
})();
