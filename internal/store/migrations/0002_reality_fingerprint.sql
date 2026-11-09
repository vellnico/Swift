-- REALITY clients imitate Firefox instead of Chrome: the ClientHello Xray sends
-- as chrome is cut by Russian DPI on port 443. Hand-edited profiles keep theirs.
UPDATE profiles
SET params_json = json_set(params_json, '$.fingerprint', 'firefox')
WHERE kind IN ('vless_reality_vision', 'vless_reality_xhttp')
  AND manual = 0
  AND json_extract(params_json, '$.fingerprint') = 'chrome';
