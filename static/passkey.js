"use strict";
/* Fingerprint / passkey sign-in for De Go Salon.

   The browser talks to the server twice for every ceremony: once for the
   challenge, and once with the signed answer. The password form remains the
   fallback on every page, so a browser without WebAuthn loses nothing. */
(function () {
    "use strict";

    var supported = typeof window !== "undefined" &&
        window.isSecureContext &&
        typeof window.PublicKeyCredential !== "undefined" &&
        typeof navigator !== "undefined" &&
        navigator.credentials &&
        typeof navigator.credentials.create === "function" &&
        typeof navigator.credentials.get === "function";

    function encode(buffer) {
        var bytes = new Uint8Array(buffer);
        var binary = "";
        var chunk = 0x8000;
        for (var i = 0; i < bytes.length; i += chunk) {
            binary += String.fromCharCode.apply(null, bytes.subarray(i, i + chunk));
        }
        return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
    }

    function decode(value) {
        var text = String(value || "").replace(/-/g, "+").replace(/_/g, "/");
        while (text.length % 4) {
            text += "=";
        }
        var binary = atob(text);
        var bytes = new Uint8Array(binary.length);
        for (var i = 0; i < binary.length; i++) {
            bytes[i] = binary.charCodeAt(i);
        }
        return bytes.buffer;
    }

    function prepareCreation(publicKey) {
        publicKey.challenge = decode(publicKey.challenge);
        if (publicKey.user && publicKey.user.id) {
            publicKey.user.id = decode(publicKey.user.id);
        }
        if (publicKey.excludeCredentials) {
            publicKey.excludeCredentials.forEach(function (item) {
                item.id = decode(item.id);
            });
        }
        return publicKey;
    }

    function prepareRequest(publicKey) {
        publicKey.challenge = decode(publicKey.challenge);
        if (publicKey.allowCredentials) {
            publicKey.allowCredentials.forEach(function (item) {
                item.id = decode(item.id);
            });
        }
        return publicKey;
    }

    function creationPayload(credential) {
        var response = credential.response;
        return {
            id: credential.id,
            rawId: encode(credential.rawId),
            type: credential.type,
            authenticatorAttachment: credential.authenticatorAttachment || undefined,
            clientExtensionResults: credential.getClientExtensionResults ? credential.getClientExtensionResults() : {},
            response: {
                clientDataJSON: encode(response.clientDataJSON),
                attestationObject: encode(response.attestationObject),
                transports: response.getTransports ? response.getTransports() : []
            }
        };
    }

    function assertionPayload(credential) {
        var response = credential.response;
        return {
            id: credential.id,
            rawId: encode(credential.rawId),
            type: credential.type,
            authenticatorAttachment: credential.authenticatorAttachment || undefined,
            clientExtensionResults: credential.getClientExtensionResults ? credential.getClientExtensionResults() : {},
            response: {
                clientDataJSON: encode(response.clientDataJSON),
                authenticatorData: encode(response.authenticatorData),
                signature: encode(response.signature),
                userHandle: response.userHandle ? encode(response.userHandle) : undefined
            }
        };
    }

    function postJSON(url, body, token) {
        var headers = { "Accept": "application/json" };
        if (body !== null) {
            headers["Content-Type"] = "application/json";
        }
        if (token) {
            headers["X-CSRF-Token"] = token;
        }
        return fetch(url, {
            method: "POST",
            credentials: "same-origin",
            headers: headers,
            body: body === null ? null : JSON.stringify(body)
        }).then(function (response) {
            return response.text().then(function (text) {
                var data = null;
                try {
                    data = text ? JSON.parse(text) : null;
                } catch (ignore) {
                    data = null;
                }
                return { ok: response.ok, status: response.status, data: data };
            });
        });
    }

    function messageFor(error, fallback) {
        if (!error) {
            return fallback;
        }
        if (error.name === "NotAllowedError") {
            return "Fingerprint sign-in was cancelled or not recognised. Try again, or use your password.";
        }
        if (error.name === "SecurityError" || error.name === "NotSupportedError") {
            return "This browser cannot use fingerprint sign-in on this site. Use your password instead.";
        }
        if (error.name === "InvalidStateError") {
            return "This device is already registered for fingerprint sign-in.";
        }
        return fallback;
    }

    function setStatus(element, text, state) {
        if (!element) {
            return;
        }
        element.textContent = text || "";
        if (state) {
            element.setAttribute("data-state", state);
        } else {
            element.removeAttribute("data-state");
        }
    }

    function safeNext(value) {
        var next = String(value || "").trim();
        if (!next || next.charAt(0) !== "/" || next.charAt(1) === "/" ||
            next.indexOf("\\") !== -1 || next.indexOf("/customer/") !== 0 && next.indexOf("/barber/") !== 0) {
            return "";
        }
        return next;
    }

    function wireLogin(button) {
        var wrapper = button.closest("[data-passkey-login-wrap]");
        var status = wrapper ? wrapper.querySelector("[data-passkey-status]") : null;
        var role = button.getAttribute("data-role") === "barber" ? "barber" : "customer";
        var next = safeNext(button.getAttribute("data-next"));

        wrapper.hidden = false;

        button.addEventListener("click", function () {
            button.disabled = true;
            setStatus(status, "Waiting for your fingerprint…");
            postJSON("/" + role + "/passkey/login/begin", {}, null)
                .then(function (started) {
                    if (!started.ok || !started.data || !started.data.publicKey) {
                        throw { name: "ServerError", message: started.data && started.data.error };
                    }
                    return navigator.credentials.get({ publicKey: prepareRequest(started.data.publicKey) });
                })
                .then(function (credential) {
                    return postJSON("/" + role + "/passkey/login/finish", assertionPayload(credential), null);
                })
                .then(function (finished) {
                    if (!finished.ok) {
                        throw { name: "ServerError", message: finished.data && finished.data.error };
                    }
                    var destination = next || (finished.data && finished.data.redirect) ||
                        (role === "barber" ? "/barber/dashboard" : "/customer/dashboard");
                    window.location.assign(destination);
                })
                .catch(function (error) {
                    setStatus(status, messageFor(error, (error && error.message) || "Fingerprint sign-in did not complete. Use your password instead."), "error");
                })
                .then(function () {
                    button.disabled = false;
                });
        });
    }

    function wireEnrolment(section) {
        var button = section.querySelector("[data-passkey-enroll]");
        var status = section.querySelector("[data-passkey-status]");
        var role = section.getAttribute("data-role") === "barber" ? "barber" : "customer";
        var token = section.getAttribute("data-csrf") || "";

        section.hidden = false;

        button.addEventListener("click", function () {
            button.disabled = true;
            setStatus(status, "Waiting for your fingerprint…");
            postJSON("/" + role + "/passkey/register/begin", null, token)
                .then(function (started) {
                    if (!started.ok || !started.data || !started.data.publicKey) {
                        throw { name: "ServerError", message: started.data && started.data.error };
                    }
                    return navigator.credentials.create({ publicKey: prepareCreation(started.data.publicKey) });
                })
                .then(function (credential) {
                    return postJSON("/" + role + "/passkey/register/finish", creationPayload(credential), token);
                })
                .then(function (finished) {
                    if (!finished.ok) {
                        throw { name: "ServerError", message: finished.data && finished.data.error };
                    }
                    button.textContent = "Add another device";
                    setStatus(status, "Fingerprint sign-in is enabled on this device.", "ok");
                })
                .catch(function (error) {
                    setStatus(status, messageFor(error, (error && error.message) || "Fingerprint sign-in could not be set up. Try again later."), "error");
                })
                .then(function () {
                    button.disabled = false;
                });
        });
    }

    if (!supported) {
        return;
    }

    document.querySelectorAll("[data-passkey-login]").forEach(wireLogin);
    document.querySelectorAll("[data-passkey-settings]").forEach(wireEnrolment);
})();
