import React, { useCallback, useEffect, useMemo, memo } from "react";
import { Search } from "lucide-react";

import { Input } from "./ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger } from "./ui/select";

interface SelectOption<TValue extends string = string> {
  value: TValue;
  label: Exclude<React.ReactNode, null | undefined>;
  searchLabel?: string;
}

interface SearchSelectProps<TValue extends string = string> {
  className?: string;
  options: SelectOption<TValue>[];
  value?: TValue;
  onValueChange?: (value: TValue) => void;
  placeholder?: string;
  searchPlaceholder?: string;
  disabled?: boolean;
}

const SearchSelect = memo(function SearchSelect<TValue extends string = string>({
  className,
  options,
  value,
  onValueChange,
  placeholder,
  searchPlaceholder,
  disabled = false,
}: SearchSelectProps<TValue>) {
  const [searchQuery, setSearchQuery] = React.useState<string>("");
  const [isOpen, setIsOpen] = React.useState(false);
  const searchInputRef = React.useRef<HTMLInputElement>(null);

  const selectedOption = useMemo(() => {
    if (!value) return undefined;
    return options.find(option => option.value === value);
  }, [options, value]);

  useEffect(() => {
    if (isOpen && searchInputRef.current) {
      const timeoutId = setTimeout(() => {
        searchInputRef.current!.focus();
      }, 0);

      return () => clearTimeout(timeoutId);
    }
  }, [isOpen]);

  // 搜索变化处理函数 - 移除防抖以解决焦点问题
  const handleSearchChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    e.stopPropagation();
    setSearchQuery(e.target.value);
  }, []);

  // Memoized filtered options to prevent unnecessary re-renders
  const filteredOptions = useMemo(() => {
  return options.filter((option) => {
    const searchText = (option.searchLabel ?? (typeof option.label === 'string' ? option.label : '')) as string;
    return searchText.toLowerCase().includes(searchQuery.toLowerCase());
  });
}, [options, searchQuery]);

  return (
    <div className={className}>
      <Select
        value={value}
        onValueChange={onValueChange}
        disabled={disabled}
        open={isOpen}
        onOpenChange={setIsOpen}
      >
        <SelectTrigger className="w-full">
          {selectedOption ? (
            <div className="flex items-center">{selectedOption.label}</div>
          ) : (
            <div className="flex items-center text-muted-foreground">
              <Search className="mr-2 h-4 w-4" />
              {placeholder || "Select an option"}
            </div>
          )}
        </SelectTrigger>
        <SelectContent className="w-full">
          <div className="p-2 border-b sticky top-0 bg-background z-10">
            <Input
              placeholder={searchPlaceholder || "Search..."}
              className="w-full"
              value={searchQuery}
              onChange={handleSearchChange}
              onKeyDown={(e) => e.stopPropagation()}
              disabled={disabled}
              ref={searchInputRef}
            />
          </div>
          {filteredOptions.length > 0 ? (
            filteredOptions.map((option: SelectOption<TValue>) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))
          ) : (
            <div className="p-4 text-center text-muted-foreground">
              No options found
            </div>
          )}
        </SelectContent>
      </Select>
    </div>
  );
});

export default SearchSelect;