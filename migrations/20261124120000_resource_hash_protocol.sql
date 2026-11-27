UPDATE resources r
SET provider_hash = encode(sha256(convert_to(concat_ws(';;',
      CASE WHEN r.direction = 'provides' THEN p.name ELSE r.consumed_provider END,
      'rest', rr.endpoint, rr.method, rr.response_status_code), 'UTF8')), 'hex'),
    consumer_hash = CASE WHEN r.direction = 'consumes' THEN encode(sha256(convert_to(concat_ws(';;',
      p.name, r.consumed_provider,
      'rest', rr.endpoint, rr.method, rr.response_status_code), 'UTF8')), 'hex') END
FROM rest_resources rr, participants p
WHERE rr.resource_id = r.id AND p.id = r.participant_id;
