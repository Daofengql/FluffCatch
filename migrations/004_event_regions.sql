ALTER TABLE events
  ADD COLUMN province_code varchar(12) NULL AFTER location,
  ADD COLUMN province_name varchar(64) NULL AFTER province_code,
  ADD COLUMN city_code varchar(12) NULL AFTER province_name,
  ADD COLUMN city_name varchar(64) NULL AFTER city_code,
  ADD KEY events_region_index (province_code, city_code);
