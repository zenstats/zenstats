import React, { useState, useEffect, useCallback, useMemo } from "react";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";

import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@radix-ui/react-popover";
import { useNavigate, useParams } from 'react-router-dom';
import type { DateRange } from "react-day-picker";
import { Calendar } from "@/components/ui/calendar";
import { Button } from "@/components/ui/button";
import { RefreshCw, ChevronDown, Settings, Link } from "lucide-react";
import axios, { type BaseResponse } from "@utils/axios";
import qs from 'qs';
import dayjs from 'dayjs'
import type { RankItem, Site, StatsRequest, TimeRangeVisitor, TopStats } from "../types/interfaces";
import TimeRange from "./components/time-range";
import TopStatsComponent from "./components/top-stats";
import TableRank from "./components/table-rank";
import Devices from "./components/device";
import Forbidden from "@components/403"

export default function StatePage() {
  const navigate = useNavigate();
  const [selectedDateRange, setSelectedDateRange] = useState<DateRange>({ from: undefined, to: undefined });
  const [queryTime, setQueryTime] = useState<Date>(new Date());
  const [isDatePickerOpen, setIsDatePickerOpen] = useState(false);
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const [selectdOptionName, setSelectdOptionName] = useState<string>("Today");
  const [sites, setSites] = useState([] as Site[]);
  const { domain } = useParams();
  const [hasAccess, setHasAccess] = useState<boolean | null>(null);


  const [query, setQuery] = useState<StatsRequest>({
    period: "day",
  });

  const api = useMemo(() => ({
    // 获取今日流量数据
    getTopStats: async (dateRange: StatsRequest) => {
      const response = await axios.get<BaseResponse<TopStats>>(domain + "/top_stats", {
        params: dateRange
      });
      return response.data;
    },
    getTimeRangeVisitor: async (dateRange: StatsRequest) => {
      const response = await axios.get<BaseResponse<TimeRangeVisitor[]>>(domain + "/curve", {
        params: dateRange
      });
      return response.data;
    },
    getPageRank: async (dateRange: StatsRequest) => {
      const response = await axios.get<BaseResponse<RankItem[]>>(domain + "/page_rank", {
        params: dateRange
      });
      return response.data;
    },
    getDeviceRank: async (dateRange: StatsRequest) => {
      const response = await axios.get<BaseResponse<RankItem[]>>(domain + "/device_rank", {
        params: dateRange
      });
      return response.data;
    },
    getSourceRank: async (dateRange: StatsRequest) => {
      const response = await axios.get<BaseResponse<RankItem[]>>(domain + "/source_rank", {
        params: dateRange
      });
      return response.data;
    },
    getSiteList: async () => {
      const response = await axios.get<BaseResponse<Site[]>>("/sites");
      return response.data;
    },
  }), [domain]);

  const fetchAllData = useCallback(() => {
    const query = qs.parse(window.location.search, { ignoreQueryPrefix: true });
    const period = ["realtime", "day", "month", "week", "custom", "w", "m", "p7", "p14", "p30"].includes(query.period as string) ? query.period as string : "day";
    const date = query.date as string;
    const from = query.from as string;
    const to = query.to as string;

    const request: StatsRequest = {
      period: period,
    };
    // 如果date不为空
    request.date = dayjs(date).format("YYYY-MM-DD");
    if (from) {
      request.from = dayjs(from).format("YYYY-MM-DD");
    }
    if (to) {
      request.to = dayjs(to).format("YYYY-MM-DD");
    }
    request.refresh = dayjs().toDate();
    setQuery(request);
  }, []);

  const handleDayClick = (day: Date) => {
    const { from, to } = selectedDateRange;
    const newRange = { ...selectedDateRange };

    if (!from && !to) {
      newRange.from = day;
      // newRange.to = day;
    } else if (!from && to) {
      newRange.from = day;
      if (day > to) {
        newRange.to = day;
      }
    } else if (from && !to) {
      newRange.to = day;
      if (day < from) {
        newRange.from = day;
      }
    } else if (from && to) {
      if (day < from) {
        newRange.from = day;
      }
      if (day > to) {
        newRange.to = day;
      }
    }

    setSelectedDateRange(newRange);

  }

  // 设置url并触发查询
  const handleChangeOoption = useCallback((option: string) => {
    const now = dayjs().toDate();
    const yesterday = dayjs().subtract(1, 'day').toDate();
    let date: Date | undefined;

    if (option === "yesterday") {
      date = yesterday
      option = 'day'
    } else {
      date = now
    }

    if (option !== "custom") {
      setSelectedDateRange({ from: undefined, to: undefined })
    } else {
      if (dayjs(selectedDateRange.from).isSame(selectedDateRange.to, 'day')) {
        date = selectedDateRange.from;
        option = 'day'
      }
    }
    const params: StatsRequest = { period: option };
    if (option === "custom") {
      params.from = dayjs(selectedDateRange.from).format('YYYY-MM-DD');
      params.to = dayjs(selectedDateRange.to).format('YYYY-MM-DD');
    } else {
      params.date = dayjs(date).format('YYYY-MM-DD');
    }

    const queryString = qs.stringify(params);
    window.history.pushState({}, '', `${window.location.pathname}?${queryString.toString()}`);

    setIsDropdownOpen(false);
    // 触发查询
    setQueryTime(new Date());
  }, [selectedDateRange]);

  // 监听日期选择器的变化
  useEffect(() => {
    if (selectedDateRange.from && selectedDateRange.to) {
      if (selectedDateRange.from == selectedDateRange.to) {
        handleChangeOoption("day")
      } else {
        handleChangeOoption("custom")
      }
      setIsDatePickerOpen(false)
    }
  }, [handleChangeOoption, selectedDateRange]);

  useEffect(() => {
    const fetchSiteList = async () => {
      try {
        const response = await api.getSiteList();
        if (response.data) {
          setSites(response.data);

          const found = response.data.some(site => site.domain === domain);
          setHasAccess(found);
        }
      } catch (error) {
        console.error('获取站点列表失败:', error);
      }
    };

    fetchSiteList();
  }, [domain, api]);

  const handleSelectdOptionName = () => {
    const query = qs.parse(window.location.search, { ignoreQueryPrefix: true });
    const period = query.period
    const date = query.date as string
    if (period === 'day') {
      if (dayjs(date).isSame(dayjs(), 'day')) {
        setSelectdOptionName("Today")
      } else {
        setSelectdOptionName(dayjs(date).format('MMM DD, YYYY'))
      }
    }
  }

  const refreshData = () => {
    setQueryTime(new Date());
  };

  useEffect(() => {
    handleSelectdOptionName()

    fetchAllData()
  }, [queryTime, fetchAllData]);

  if (hasAccess === false) {
    return (
      <Forbidden />
    );
  }

  if (hasAccess === null) {
    return (
      <div className="container mx-auto py-6 px-4 text-center">
        <p className="text-lg">正在检查访问权限...</p>
      </div>
    );
  }

  return (
    <div className="container mx-auto py-6 px-4 space-y-6">
      <div className="flex justify-between items-center mb-6">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button className="text-2xl font-bold flex items-center">
              <Link className="mr-2 h-4 w-4" />
              {domain}
              <ChevronDown className="ml-2 h-5 w-5" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent>

            {(sites.find(site => site.domain === domain)?.role === 'admin' ||
              sites.find(site => site.domain === domain)?.role === 'owner') && (
                <>
                  <DropdownMenuItem
                    onClick={() => window.location.href = `/sites/${domain}/settings`}
                    className="flex items-center"
                  >
                    <Settings className="mr-2 h-4 w-4" />
                    站点设置
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                </>
              )}
            {sites.map(site => (
              <DropdownMenuItem
                key={site.domain}
                onClick={() => {
                  navigate(`/sites/${site.domain}`)
                }}
              >
                <Link className="mr-2 h-4 w-4" />
                {site.domain}
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onClick={() => window.location.href = `/sites`}
              className="flex items-center"
            >
              View All
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <div className="flex items-center space-x-3">
          <Popover open={isDatePickerOpen}>
            <PopoverTrigger asChild>
              <span />
            </PopoverTrigger>
            <PopoverContent>
              <Calendar
                className="mt-6 ml-35 z-10"
                mode="range"
                showOutsideDays={false}
                numberOfMonths={1}
                onDayClick={handleDayClick}
                selected={selectedDateRange}
                disabled={(date) =>
                  date > new Date() || date < new Date("1900-01-01")
                }
              />
            </PopoverContent>
          </Popover>

          <DropdownMenu open={isDropdownOpen} onOpenChange={setIsDropdownOpen}>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className="flex justify-between items-center space-x-2 bg-white p-2 rounded-lg shadow-sm w-[180px] text-left transition-colors hover:bg-gray-50 focus:ring-2 focus:ring-blue-500/20"
                onClick={() => setIsDropdownOpen(!isDropdownOpen)}
              >
                <span className="text-sm font-medium">
                  {selectdOptionName}
                </span>
                <ChevronDown className="h-4 w-4" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent className="w-56 p-2 rounded-lg">
              <DropdownMenuItem
                onClick={() => { handleChangeOoption("day") }}
                className="flex items-center space-x-3"
              >
                <span>今日</span>
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => { handleChangeOoption("yesterday") }}
                className="flex items-center space-x-3"
              >
                <span>昨日</span>
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => { handleChangeOoption("realtime") }}
                className="flex items-center space-x-3"
              >
                <span>实时</span>
              </DropdownMenuItem>
              <DropdownMenuSeparator className="my-2" />
              <DropdownMenuItem
                onClick={() => { handleChangeOoption("w") }}
                className="flex items-center space-x-3"
              >
                <span>本周</span>
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => { handleChangeOoption("m") }}
                className="flex items-center space-x-3"
              >
                <span>本月</span>
              </DropdownMenuItem>

              <DropdownMenuSeparator className="my-2" />
              <DropdownMenuItem
                onClick={() => { handleChangeOoption("p7") }}
                className="flex items-center space-x-3"
              >
                <span>最近7天</span>
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => { handleChangeOoption("p14") }}
                className="flex items-center space-x-3"
              >
                <span>最近14天</span>
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => { handleChangeOoption("p30") }}
                className="flex items-center space-x-3"
              >
                <span>最近30天</span>
              </DropdownMenuItem>
              <DropdownMenuSeparator className="my-2" />
              <DropdownMenuItem
                onClick={() => {
                  setIsDatePickerOpen(true);
                  setSelectedDateRange({ from: undefined, to: undefined })
                  setIsDropdownOpen(false);
                }}
                className="flex items-center space-x-3 text-blue-600"
              >
                <span>自定义范围</span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>


          <Button variant="ghost" size="icon" onClick={refreshData}>
            <RefreshCw className="h-4 w-4" />
          </Button>
        </div>
      </div>

      <TopStatsComponent api={api.getTopStats} query={query} setQuery={setQuery} />

      <TimeRange query={query} setQuery={setQuery} api={api.getTimeRangeVisitor} />

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <TableRank
          title="来源"
          keyName="Source"
          limit={10}
          query={query}
          setQuery={setQuery}
          api={api.getSourceRank}
        />

        <TableRank
          title="热门页面"
          keyName="Page"
          limit={10}
          query={query}
          setQuery={setQuery}
          api={api.getPageRank}
        />
      </div>

      {/* 区域数据和设备数据 */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* 设备数据 */}
        <Devices query={query} setQuery={setQuery} limit={10} api={api.getDeviceRank} />
      </div>
    </div>
  );
};