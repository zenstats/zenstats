import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@components/ui/select";
import { useState } from "react";
import { Input } from "./ui/input";
import timeZones from "@/constants/time-zones.json";

type TimeZoneSelectorProps = {
  // 选择时区后的回调函数
  onChange: (...event: unknown[]) => void;
  // 默认选中的时区
  defaultValue?: string;
  value?: string;
};

export function TimeZoneSelector({
  onChange,
  defaultValue,
  value,
}: TimeZoneSelectorProps) {
  const [searchQuery, setSearchQuery] = useState("");
  const filteredTimeZones = timeZones.filter((timeZone) =>
    timeZone.label.toLowerCase().includes(searchQuery.toLowerCase()),
  );
  const initialValue = value !== undefined ? value : defaultValue;

  return (
    <Select defaultValue={initialValue} onValueChange={onChange}>
      <SelectTrigger>
        <SelectValue placeholder="Select a timezone" />
      </SelectTrigger>
      <SelectContent className="relative">
        <Input
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Search timezones"
          className="sticky top-0 z-10 bg-white mb-2"
        />
        {filteredTimeZones.map((timeZone) => (
          <SelectItem key={timeZone.value} value={timeZone.value}>
            {timeZone.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
