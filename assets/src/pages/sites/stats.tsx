import React, { useState, useEffect } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  AreaChart,
  Area,
} from "recharts";
import { PieChart, Pie, Cell } from "recharts";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import { Calendar } from "@/components/ui/calendar";
import { Button } from "@/components/ui/button";
import { RefreshCw, ChevronsUpDown } from "lucide-react";
import axios, { type BaseResponse } from "@utils/axios";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@radix-ui/react-popover";
import { useParams } from 'react-router-dom';

interface TopStats {
  pv: number;
  uv: number;
  sessions: number;
  pv_change: number;
  uv_change: number;
  session_change: number;
  avg_duration: number;
  avg_duration_change: number;
  avg_duration_format: string;
}

interface TimeRangeVisitor {
  date: string;
  uv: number;
}

interface RankItem {
  key: string;
  visits: number;
  percentage?: number;
}


interface StatsRequest {
  period: string;
  date?: string; // 日期，可选
  start_date?: string; // 开始日期，可选
  end_data?: string; // 结束日期，可选
}


const StatePage: React.FC = () => {
  const [selectedDateRange, setSelectedDateRange] = useState<{
    start: Date | null;
    end: Date | null;
  }>({ start: null, end: null });
  const [selectedOption, setSelectedOption] = useState<string>("Today");
  const [isDatePickerOpen, setIsDatePickerOpen] = useState(false);
  // 新增状态控制 DropdownMenu 的显示
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const { domain } = useParams();


  const [topStatsLoading, setTopStatsLoading] = useState(true);
  const [topStats, setTopStats] = useState<BaseResponse<TopStats> | undefined>();
  const [timeRangeVisitorLoading, setTimeRangeVisitorLoading] = useState(true);
  const [timeRangeVisitor, setTimeRangeVisitor] = useState<BaseResponse<TimeRangeVisitor[]> | undefined>();

  const [sourceRankLoading, setSourceRankLoading] = useState(true);
  const [sourceRank, setSourceRank] = useState<BaseResponse<RankItem[]> | undefined>();
  const [pageRankLoading, setPageRankLoading] = useState(true);
  const [pageRank, setPageRank] = useState<BaseResponse<RankItem[]> | undefined>();
  const [deviceRankLoading, setDeviceRankLoading] = useState(true);
  const [deviceRank, setDeviceRank] = useState<BaseResponse<RankItem[]> | undefined>();

  const api = {
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
  };


  // 模拟数据加载
  useEffect(() => {
    const fetchData = async () => {
      setTopStatsLoading(true);
      setTimeRangeVisitorLoading(true);
      try {
        const results = await Promise.allSettled([
          api.getTopStats({
            period: "T",
            date: "2025-07-02"
          }),
          api.getTimeRangeVisitor({
            period: "T",
            date: "2025-07-02"
          }),
          api.getDeviceRank({
            period: "T",
            date: "2025-07-02"
          }),
          api.getPageRank({
            period: "T",
            date: "2025-07-02"
          }),
          api.getSourceRank({
            period: "T",
            date: "2025-07-02"
          }),
        ]);
        // 处理每个结果
        results.forEach((result, index) => {
          if (result.status === "fulfilled") {
            switch (index) {
              case 0:
                setTopStats(result.value as BaseResponse<TopStats>);
                setTopStatsLoading(false);
                break;
              case 1:
                setTimeRangeVisitor(result.value as BaseResponse<TimeRangeVisitor[]>);
                setTimeRangeVisitorLoading(false);
                break;
              case 2:
                setDeviceRank(result.value as BaseResponse<RankItem[]>);
                setDeviceRankLoading(false);
                break;
              case 3:
                setPageRank(result.value as BaseResponse<RankItem[]>);
                setPageRankLoading(false);
                break;
              case 4:
                setSourceRank(result.value as BaseResponse<RankItem[]>);
                setSourceRankLoading(false);
                break;
            }
          } else {
            console.error(`请求失败: ${index}`, result.reason);
          }
        });
      } catch (error) {
        console.error("数据加载失败:", error);
      }
    };

    fetchData();
  }, [selectedDateRange]);


  // 刷新数据
  const refreshData = () => {

    setTopStatsLoading(true);
    setTimeout(() => {
      setTopStatsLoading(false);

    }, 1000);
  };

  // 格式化日期范围显示
  const formatDateRange = (start: Date | null, end: Date | null) => {
    if (!start || !end) return "Custom Range";
    const formatDate = (date: Date) =>
      `${date.getFullYear()}-${(date.getMonth() + 1).toString().padStart(2, "0")}-${date.getDate().toString().padStart(2, "0")}`;
    return `${formatDate(start)} - ${formatDate(end)}`;
  };

  return (
    <div className="container mx-auto py-6 px-4 space-y-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">网站流量分析</h1>
        <div className="flex items-center space-x-3">
          {/* dropdown-menu */}
          <DropdownMenu open={isDropdownOpen} onOpenChange={setIsDropdownOpen}>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className="flex items-center space-x-2 bg-white p-1 rounded-md shadow-sm w-[180px] text-left"
                onClick={() => setIsDropdownOpen(!isDropdownOpen)}
              >
                <span className="text-sm font-medium">
                  {selectedOption === "cr"
                    ? formatDateRange(
                      selectedDateRange.start,
                      selectedDateRange.end,
                    )
                    : selectedOption === "T"
                      ? "Today"
                      : selectedOption === "Y"
                        ? "Yesterday"
                        : selectedOption === "R"
                          ? "Realtime"
                          : selectedOption === "p7"
                            ? "Last 7 Days"
                            : selectedOption === "p14"
                              ? "Last 14 Days"
                              : selectedOption === "p30"
                                ? "Last 30 Days"
                                : "Custom Range"}
                </span>
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent className="w-56">
              <DropdownMenuItem
                onClick={() => {
                  setSelectedOption("T");
                }}
              >
                Today
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => {
                  setSelectedOption("Y");
                  const yesterday = new Date();
                  yesterday.setDate(yesterday.getDate() - 1);
                  setSelectedDateRange({
                    start: yesterday,
                    end: yesterday,
                  });
                }}
              >
                Yesterday
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => {
                  setSelectedOption("R");
                }}
              >
                Realtime
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onClick={() => {
                  setSelectedOption("p7");
                  const now = new Date();
                  const sevenDaysAgo = new Date(now);
                  sevenDaysAgo.setDate(now.getDate() - 7);
                  setSelectedDateRange({
                    start: sevenDaysAgo,
                    end: now,
                  });
                  setIsDropdownOpen(false); // 选择后关闭 DropdownMenu
                }}
              >
                Last 7 Days
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => {
                  setSelectedOption("p14");
                  const now = new Date();
                  const fourteenDaysAgo = new Date(now);
                  fourteenDaysAgo.setDate(now.getDate() - 14);
                  setSelectedDateRange({
                    start: fourteenDaysAgo,
                    end: now,
                  });
                }}
              >
                Last 14 Days
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => {
                  setSelectedOption("p30");
                  const now = new Date();
                  const thirtyDaysAgo = new Date(now);
                  thirtyDaysAgo.setDate(now.getDate() - 30);
                  setSelectedDateRange({
                    start: thirtyDaysAgo,
                    end: now,
                  });
                }}
              >
                Last 30 Days
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onClick={() => {
                  setIsDatePickerOpen(true);
                  setIsDropdownOpen(false); // 点击 Custom Range 后关闭 DropdownMenu
                  console.log("Custom Range clicked");
                }}
              >
                Custom Range
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <Button variant="ghost" size="icon" onClick={refreshData}>
            <RefreshCw className="h-4 w-4" />
          </Button>
        </div>
      </div>

      <div className="block absolute right-10 top-30">
        <Popover open={isDatePickerOpen}>
          <PopoverTrigger asChild>
            <span />
          </PopoverTrigger>
          <PopoverContent>
            <Calendar
              mode="range"
              numberOfMonths={2}
            // onSelect={() => setIsDatePickerOpen(!isDatePickerOpen)}
            />
          </PopoverContent>
        </Popover>
      </div>

      <Card className="grid grid-cols-1 md:grid-cols-5 gap-4 ">
        {topStatsLoading ? (
          <Skeleton className="col-span-5 h-30 w-full" />
        ) : (
          <>
            <div className="col-span-1 md:col-span-1 p-4 border-r border-gray-200 dark:border-gray-700">
              <CardHeader className="p-0 pb-2">
                <CardTitle className="text-sm font-medium text-gray-500">
                  总访问量 (UV)
                </CardTitle>
              </CardHeader>
              <CardContent className="p-0 pt-0">
                <div className="text-2xl font-bold">
                  {topStats?.data.uv}
                </div>
                <p className="text-xs text-green-500 mt-1 flex items-center">
                  <ChevronsUpDown className="h-3 w-3 mr-1" />
                  {topStats?.data.uv_change}%
                </p>
              </CardContent>
            </div>

            <div className="col-span-1 md:col-span-1 p-4 border-r border-gray-200 dark:border-gray-700">
              <CardHeader className="p-0 pb-2">
                <CardTitle className="text-sm font-medium text-gray-500">
                  总浏览量 (PV)
                </CardTitle>
              </CardHeader>
              <CardContent className="p-0 pt-0">
                <div className="text-lg md:text-2xl font-bold">
                  {topStats?.data.pv}
                </div>
                <p className="text-xs text-green-500 mt-1 flex items-center">
                  <ChevronsUpDown className="h-3 w-3 mr-1" />
                  {topStats?.data.pv_change}%
                </p>
              </CardContent>
            </div>

            <div className="col-span-1 md:col-span-1 p-4 border-r border-gray-200 dark:border-gray-700">
              <CardHeader className="p-0 pb-2">
                <CardTitle className="text-sm font-medium text-gray-500">
                  跳出率
                </CardTitle>
              </CardHeader>
              <CardContent className="p-0 pt-0">
                <div className="text-2xl font-bold">
                  123
                </div>
                <p className="text-xs text-green-500 mt-1 flex items-center">
                  <ChevronsUpDown className="h-3 w-3 mr-1" />
                  8.3%
                </p>
              </CardContent>
            </div>

            <div className="col-span-1 md:col-span-1 p-4 border-r border-gray-200 dark:border-gray-700">
              <CardHeader className="p-0 pb-2">
                <CardTitle className="text-sm font-medium text-gray-500">
                  平均访问时长
                </CardTitle>
              </CardHeader>
              <CardContent className="p-0 pt-0">
                <div className="text-2xl font-bold">
                  {topStats?.data.avg_duration_format}
                </div>
                <p className="text-xs text-green-500 mt-1 flex items-center">
                  <ChevronsUpDown className="h-3 w-3 mr-1" />
                  {topStats?.data.avg_duration_change}%
                </p>
              </CardContent>
            </div>

            <div className="col-span-1 md:col-span-1 p-4">
              <CardHeader className="p-0 pb-2">
                <CardTitle className="text-sm font-medium text-gray-500">
                  新访客比例
                </CardTitle>
              </CardHeader>
              <CardContent className="p-0 pt-0">

                <div className="text-2xl font-bold">
                  {topStats?.data.avg_duration}
                </div>
                <p className="text-xs text-green-500 mt-1 flex items-center">
                  <ChevronsUpDown className="h-3 w-3 mr-1" />
                  8.3%
                </p>
              </CardContent>
            </div>
          </>
        )
        }
      </Card>

      {/* 时间段UV曲线图 */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <div>
            <CardTitle>访问趋势</CardTitle>
            <CardDescription>今日各时段UV访问量</CardDescription>
          </div>
          <Tabs defaultValue="day" className="w-[300px]">
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="day">日</TabsTrigger>
              <TabsTrigger value="week">周</TabsTrigger>
              <TabsTrigger value="month">月</TabsTrigger>
            </TabsList>
          </Tabs>
        </CardHeader>
        <CardContent>
          <div className="h-[300px]">
            {timeRangeVisitorLoading ? (
              <Skeleton className="h-full w-full rounded-md" />
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart
                  data={timeRangeVisitor?.data}
                  margin={{ top: 10, right: 30, left: 0, bottom: 0 }}
                >
                  <defs>
                    <linearGradient id="colorUv" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.8} />
                      <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid
                    strokeDasharray="3 3"
                    vertical={false}
                    stroke="#f1f5f9"
                  />
                  <XAxis
                    dataKey="time"
                    tick={{ fontSize: 12 }}
                    axisLine={false}
                    tickLine={false}
                  />
                  <YAxis
                    tick={{ fontSize: 12 }}
                    axisLine={false}
                    tickLine={false}
                  />
                  <Tooltip
                    contentStyle={{
                      borderRadius: "8px",
                      border: "none",
                      boxShadow: "0 4px 12px rgba(0,0,0,0.1)",
                    }}
                    formatter={(value) => [`${value}`, "访问量"]}
                  />
                  <Area
                    type="monotone"
                    dataKey="uv"
                    stroke="#3b82f6"
                    fillOpacity={1}
                    fill="url(#colorUv)"
                  />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Top Source 和 Top Pages */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Top Source */}
        <Card>
          <CardHeader>
            <CardTitle>流量来源分布</CardTitle>
            <CardDescription>访问来源渠道占比</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-[300px] flex items-center justify-center">
              {sourceRankLoading ? (
                <Skeleton className="h-full w-full rounded-md" />
              ) : (
                <div className="flex flex-col md:flex-row items-center justify-between w-full h-full">
                  <div className="w-full md:w-1/2 h-64 flex items-center justify-center">
                    <ResponsiveContainer width="100%" height="100%">
                      <PieChart>
                        <Pie
                          data={sourceRank?.data}
                          cx="50%"
                          cy="50%"
                          dataKey="visits"
                          nameKey="key"
                          innerRadius={60}
                          outerRadius={80}
                          paddingAngle={2}
                        >
                          {sourceRank?.data.map((entry, index) => (
                            <Cell
                              key={`cell-${index}`}
                              fill={`hsl(${index * 70}, 70%, 50%)`}
                            />
                          ))}
                        </Pie>
                        <Tooltip />
                        {/* <Tooltip formatter={(value) => [`${value}%`, "占比"]} /> */}
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                  <div className="w-full md:w-1/2 mt-4 md:mt-0">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>来源</TableHead>
                          <TableHead className="text-right">占比</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {sourceRank?.data.map((source, index) => (
                          <TableRow key={index}>
                            <TableCell className="font-medium">
                              {source.key}
                            </TableCell>
                            <TableCell className="text-right">
                              {source.percentage}%
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        {/* Top Pages */}
        <Card>
          <CardHeader>
            <CardTitle>热门页面</CardTitle>
            <CardDescription>访问量最高的页面</CardDescription>
          </CardHeader>
          <CardContent>
            {pageRankLoading ? (
              <div className="space-y-4">
                {[1, 2, 3, 4, 5].map((i) => (
                  <div key={i} className="flex justify-between items-center">
                    <Skeleton className="h-5 w-3/5" />
                    <Skeleton className="h-5 w-1/5" />
                  </div>
                ))}
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>页面</TableHead>
                    <TableHead className="text-right">访问量</TableHead>
                    <TableHead className="text-right">占比</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {pageRank?.data.map((page, index) => (
                    <TableRow key={index}>
                      <TableCell className="font-medium">{page.key}</TableCell>
                      <TableCell className="text-right">
                        {page.visits.toLocaleString()}
                      </TableCell>
                      <TableCell className="text-right">
                        {page.percentage}%
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>

      {/* 区域数据和设备数据 */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* 区域数据 */}
        <Card>
          <CardHeader>
            <CardTitle>区域分布</CardTitle>
            <CardDescription>国内各地区访问量</CardDescription>
          </CardHeader>
          <CardContent></CardContent>
        </Card>

        {/* 设备数据 */}
        <Card>
          <CardHeader>
            <CardTitle>设备分布</CardTitle>
            <CardDescription>访问设备类型占比</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-[300px] flex items-center justify-center">
              {deviceRankLoading ? (
                <Skeleton className="h-full w-full rounded-md" />
              ) : (
                <div className="w-full h-full flex flex-col md:flex-row items-center justify-around">
                  <div className="w-1/2 h-64 flex items-center justify-center">
                    <ResponsiveContainer width="100%" height="100%">
                      <PieChart>
                        <Pie
                          data={deviceRank?.data}
                          cx="50%"
                          cy="50%"
                          dataKey="visits"
                          nameKey="key"
                          innerRadius={60}
                          outerRadius={80}
                          paddingAngle={2}
                        >
                          {deviceRank?.data.map((entry, index) => (
                            <Cell
                              key={`cell-${index}`}
                              fill={`hsl(${index * 70}, 70%, 50%)`}
                            />
                          ))}
                        </Pie>
                        <Tooltip />
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                  <div className="w-1/2 h-full flex flex-col justify-center">
                    {deviceRank?.data.map((device, index) => (
                      <div key={index} className="flex items-center mb-6">
                        <div
                          className="w-4 h-4 rounded-full mr-3"
                        ></div>
                        <div className="flex-1">
                          <div className="flex justify-between mb-1">
                            <span className="text-sm font-medium">
                              {device.key}
                            </span>
                            <span className="text-sm font-medium">
                              {device.percentage}%
                            </span>
                          </div>
                          <div className="w-full bg-gray-200 rounded-full h-2">
                            <div
                              className="bg-blue-600 h-2 rounded-full"
                              style={{ width: `${device.percentage}%` }}
                            ></div>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
};

export default StatePage;
