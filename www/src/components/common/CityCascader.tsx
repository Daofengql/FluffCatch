import { Box, FormControl, FormHelperText, InputLabel, MenuItem, Select, type SelectChangeEvent } from '@mui/material';
import citiesData from 'china-division/dist/cities.json';
import { useEffect, useMemo, useState } from 'react';

type Province = {
  code: string;
  name: string;
};

type City = {
  code: string;
  name: string;
  provinceCode: string;
};

export type CityValue = {
  cityCode: string;
  cityName: string;
  provinceCode: string;
  provinceName: string;
};

type CityCascaderProps = {
  value: CityValue;
  onChange: (value: CityValue) => void;
  disabled?: boolean;
  helperText?: string;
  required?: boolean;
  size?: 'small' | 'medium';
};

export const provinces: Province[] = [
  { code: '11', name: '北京市' },
  { code: '12', name: '天津市' },
  { code: '13', name: '河北省' },
  { code: '14', name: '山西省' },
  { code: '15', name: '内蒙古自治区' },
  { code: '21', name: '辽宁省' },
  { code: '22', name: '吉林省' },
  { code: '23', name: '黑龙江省' },
  { code: '31', name: '上海市' },
  { code: '32', name: '江苏省' },
  { code: '33', name: '浙江省' },
  { code: '34', name: '安徽省' },
  { code: '35', name: '福建省' },
  { code: '36', name: '江西省' },
  { code: '37', name: '山东省' },
  { code: '41', name: '河南省' },
  { code: '42', name: '湖北省' },
  { code: '43', name: '湖南省' },
  { code: '44', name: '广东省' },
  { code: '45', name: '广西壮族自治区' },
  { code: '46', name: '海南省' },
  { code: '50', name: '重庆市' },
  { code: '51', name: '四川省' },
  { code: '52', name: '贵州省' },
  { code: '53', name: '云南省' },
  { code: '54', name: '西藏自治区' },
  { code: '61', name: '陕西省' },
  { code: '62', name: '甘肃省' },
  { code: '63', name: '青海省' },
  { code: '64', name: '宁夏回族自治区' },
  { code: '65', name: '新疆维吾尔自治区' },
  { code: '71', name: '台湾省' },
  { code: '81', name: '香港特别行政区' },
  { code: '82', name: '澳门特别行政区' }
];

const cities = citiesData as City[];

const emptyValue: CityValue = {
  cityCode: '',
  cityName: '',
  provinceCode: '',
  provinceName: ''
};

export function CityCascader({ disabled = false, helperText, onChange, required = false, size = 'small', value }: CityCascaderProps) {
  const [provinceCode, setProvinceCode] = useState(value.provinceCode || '');
  const [cityCode, setCityCode] = useState(value.cityCode || '');

  useEffect(() => {
    setProvinceCode(value.provinceCode || '');
    setCityCode(value.cityCode || '');
  }, [value.cityCode, value.provinceCode]);

  const cityOptions = useMemo(() => {
    if (!provinceCode) return [];
    return cities.filter((city) => city.provinceCode === provinceCode);
  }, [provinceCode]);

  function handleProvinceChange(event: SelectChangeEvent) {
    const nextProvinceCode = event.target.value;
    const province = provinces.find((item) => item.code === nextProvinceCode);
    setProvinceCode(nextProvinceCode);
    setCityCode('');
    onChange(province ? { ...emptyValue, provinceCode: province.code, provinceName: province.name } : emptyValue);
  }

  function handleCityChange(event: SelectChangeEvent) {
    const nextCityCode = event.target.value;
    const province = provinces.find((item) => item.code === provinceCode);
    const city = cities.find((item) => item.code === nextCityCode);
    setCityCode(nextCityCode);
    onChange({
      cityCode: city?.code || '',
      cityName: city?.name || '',
      provinceCode: province?.code || '',
      provinceName: province?.name || ''
    });
  }

  return (
    <Box sx={{ width: '100%' }}>
      <Box sx={{ display: 'flex', gap: 1 }}>
        <FormControl disabled={disabled} required={required} size={size} sx={{ flex: 1, minWidth: 120 }}>
          <InputLabel shrink>省份</InputLabel>
          <Select displayEmpty label="省份" notched onChange={handleProvinceChange} value={provinceCode}>
            <MenuItem value="">
              <em>选择省份</em>
            </MenuItem>
            {provinces.map((province) => (
              <MenuItem key={province.code} value={province.code}>
                {province.name}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <FormControl disabled={disabled || !provinceCode} required={required} size={size} sx={{ flex: 1, minWidth: 120 }}>
          <InputLabel shrink>城市</InputLabel>
          <Select displayEmpty label="城市" notched onChange={handleCityChange} value={cityCode}>
            <MenuItem value="">
              <em>选择城市</em>
            </MenuItem>
            {cityOptions.map((city) => (
              <MenuItem key={city.code} value={city.code}>
                {city.name}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      </Box>
      {helperText && <FormHelperText sx={{ ml: 1.5 }}>{helperText}</FormHelperText>}
    </Box>
  );
}

export function regionLabel(value: Partial<CityValue>) {
  return [value.provinceName, value.cityName].filter(Boolean).join(' ');
}
