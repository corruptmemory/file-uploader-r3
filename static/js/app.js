(function() {
    "use strict";

    // --- Cookie Utilities ---

    function getCookie(name) {
        var cookies = document.cookie.split(";");
        for (var i = 0; i < cookies.length; i++) {
            var c = cookies[i].trim();
            if (c.indexOf(name + "=") === 0) {
                return decodeURIComponent(c.substring(name.length + 1));
            }
        }
        return null;
    }

    // --- Session Management ---

    var sessionTimerInterval = null;

    function checkSession() {
        var expiresStr = getCookie("session-expires");
        if (!expiresStr) {
            return -1;
        }
        var expiresMs = Date.parse(expiresStr);
        if (isNaN(expiresMs)) {
            return -1;
        }
        var nowMs = Date.now();
        return Math.floor((expiresMs - nowMs) / 1000);
    }

    function extendSession() {
        fetch(getPrefix() + "/api/extend", {
            method: "GET",
            credentials: "same-origin"
        }).catch(function() {
            // Silently ignore extension failures
        });
    }

    // Expose globally for the session modal button
    window.extendSession = extendSession;

    function showSessionModal() {
        var modal = document.getElementById("session-modal");
        if (modal) {
            modal.classList.add("active");
        }
    }

    function hideSessionModal() {
        var modal = document.getElementById("session-modal");
        if (modal) {
            modal.classList.remove("active");
        }
    }

    // Expose globally for the session modal button
    window.hideSessionModal = hideSessionModal;

    function startSessionTimer() {
        if (sessionTimerInterval) {
            clearInterval(sessionTimerInterval);
        }
        sessionTimerInterval = setInterval(function() {
            var remaining = checkSession();
            if (remaining <= 0) {
                clearInterval(sessionTimerInterval);
                fetch(getPrefix() + "/logout", { method: "POST", credentials: "same-origin" })
                    .finally(function() { window.location.href = getPrefix() + "/login"; });
                return;
            }
            if (remaining <= 30) {
                showSessionModal();
            } else {
                hideSessionModal();
            }
        }, 1000);
    }

    function setupAutoExtension() {
        var lastExtend = 0;
        var cooldown = 60000; // 60 seconds

        function onActivity() {
            var now = Date.now();
            if (now - lastExtend > cooldown && checkSession() > 0) {
                lastExtend = now;
                extendSession();
            }
        }

        document.addEventListener("mousedown", onActivity);
        document.addEventListener("keydown", onActivity);
        document.addEventListener("input", onActivity);
        document.addEventListener("scroll", onActivity);
    }

    // --- File Upload ---

    var MAX_FILE_SIZE = 50 * 1024 * 1024; // 50 MB

    function getPrefix() {
        var scripts = document.querySelectorAll("script[src]");
        for (var i = 0; i < scripts.length; i++) {
            var src = scripts[i].getAttribute("src");
            var idx = src.indexOf("/js/app.js");
            if (idx !== -1) {
                return src.substring(0, idx);
            }
        }
        return "";
    }

    function isCSVFile(filename) {
        return filename.toLowerCase().endsWith(".csv");
    }

    function prepFileDrop() {
        var dropZone = document.getElementById("drop-zone");
        if (!dropZone) {
            return;
        }

        var fileInput = dropZone.querySelector(".drop-zone-file-input");
        var filesDisplay = dropZone.querySelector(".drop-zone-files");
        var pickBtn = dropZone.querySelector(".drop-zone-pick-btn");
        var uploadBtn = dropZone.querySelector(".drop-zone-upload-btn");
        var clearBtn = dropZone.querySelector(".drop-zone-clear-btn");
        var messageDiv = dropZone.querySelector(".drop-zone-message");

        var selectedFiles = [];

        function showMessage(text, type) {
            messageDiv.className = "drop-zone-message " + type;
            messageDiv.textContent = text;
            messageDiv.style.display = "";
            if (type === "success") {
                messageDiv.classList.add("fade-out");
                setTimeout(function() {
                    messageDiv.style.display = "none";
                    messageDiv.classList.remove("fade-out");
                }, 5000);
            }
        }

        function hideMessage() {
            messageDiv.style.display = "none";
            messageDiv.textContent = "";
        }

        function updateDisplay() {
            filesDisplay.innerHTML = "";
            if (selectedFiles.length > 0) {
                var summary = document.createElement("div");
                summary.className = "text-sm text-muted mt-sm";
                summary.textContent = selectedFiles.length + " file" + (selectedFiles.length > 1 ? "s" : "") + " selected: ";
                var names = selectedFiles.map(function(f) { return f.name; });
                summary.textContent += names.join(", ");
                filesDisplay.appendChild(summary);
            }
            uploadBtn.disabled = selectedFiles.length === 0;
        }

        function addFiles(fileList) {
            var invalid = [];
            var tooLarge = [];
            for (var i = 0; i < fileList.length; i++) {
                var file = fileList[i];
                if (!isCSVFile(file.name)) {
                    invalid.push(file.name);
                } else if (file.size > MAX_FILE_SIZE) {
                    tooLarge.push(file.name);
                } else {
                    selectedFiles.push(file);
                }
            }
            var errors = [];
            if (invalid.length > 0) {
                errors.push("Not CSV: " + invalid.join(", "));
            }
            if (tooLarge.length > 0) {
                errors.push("Too large (max 50MB): " + tooLarge.join(", "));
            }
            if (errors.length > 0) {
                showMessage(errors.join(". "), "error");
            } else {
                hideMessage();
            }
            updateDisplay();
        }

        function clearFiles() {
            selectedFiles = [];
            fileInput.value = "";
            updateDisplay();
            hideMessage();
        }

        // Select Files button
        pickBtn.addEventListener("click", function() {
            fileInput.click();
        });

        fileInput.addEventListener("change", function() {
            addFiles(fileInput.files);
            fileInput.value = "";
        });

        // Clear button
        clearBtn.addEventListener("click", clearFiles);

        // Drag and drop
        dropZone.addEventListener("dragover", function(e) {
            e.preventDefault();
            dropZone.classList.add("drag-over");
        });

        dropZone.addEventListener("dragleave", function() {
            dropZone.classList.remove("drag-over");
        });

        dropZone.addEventListener("drop", function(e) {
            e.preventDefault();
            dropZone.classList.remove("drag-over");
            addFiles(e.dataTransfer.files);
        });

        // Upload button
        uploadBtn.addEventListener("click", function() {
            if (selectedFiles.length === 0) {
                return;
            }
            var formData = new FormData();
            selectedFiles.forEach(function(file) {
                formData.append("files", file);
            });

            uploadBtn.disabled = true;
            uploadBtn.textContent = "Uploading...";

            fetch(getPrefix() + "/upload", {
                method: "POST",
                body: formData,
                credentials: "same-origin"
            }).then(function(response) {
                return response.text().then(function(text) {
                    if (response.ok) {
                        showMessage("Files uploaded successfully", "success");
                        selectedFiles = [];
                        updateDisplay();
                    } else {
                        showMessage("Upload failed: " + text, "error");
                    }
                });
            }).catch(function() {
                showMessage("Upload failed. Please try again.", "error");
            }).finally(function() {
                uploadBtn.disabled = selectedFiles.length === 0;
                uploadBtn.textContent = "Upload";
            });
        });
    }

    // --- htmx Integration ---

    document.addEventListener("htmx:afterSwap", function(evt) {
        var target = evt.detail.target;
        if (target && target.closest && target.closest("#failure-modal")) {
            document.getElementById("failure-modal").classList.add("active");
        }
    });

    // Close modal on clicking overlay background
    document.addEventListener("click", function(evt) {
        if (evt.target.classList.contains("modal-overlay")) {
            evt.target.classList.remove("active");
        }
        if (evt.target.classList.contains("modal-close-btn")) {
            var modal = evt.target.closest(".modal-overlay");
            if (modal) {
                modal.classList.remove("active");
            }
        }
    });

    // --- Init ---

    document.addEventListener("DOMContentLoaded", function() {
        if (checkSession() > 0) {
            startSessionTimer();
        }
        setupAutoExtension();
        prepFileDrop();
    });
})();
