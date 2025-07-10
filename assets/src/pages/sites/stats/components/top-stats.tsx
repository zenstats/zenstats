import type { BaseResponse } from "@utils/axios";
import type { StatsRequest, TopStats } from "@/pages/sites/types/interfaces";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ChevronsUpDown } from "lucide-react";
import { Skeleton } from "@components/ui/skeleton";
import {
  useCallback,
  useEffect,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";

interface TopStatsProps {
  query: StatsRequest;
  setQuery?: Dispatch<SetStateAction<StatsRequest>>;
  api: (dateRange: StatsRequest) => Promise<BaseResponse<TopStats>>;
}

export default function TopStatsComponent({
  query,
  // setQuery,
  api,
}: TopStatsProps) {
  const [localLoading, setLocalLoading] = useState(false);
  const [data, setData] = useState<BaseResponse<TopStats>>();
  const fetchData = useCallback(async () => {
    try {
      setLocalLoading(true);
      const result = await api(query);
      setData(result);
    } catch (error) {
      console.error(error);
    } finally {
      setLocalLoading(false);
    }
  }, [api, query]);

  useEffect(() => {
    if (query.refresh !== undefined) {
      fetchData();
    }
  }, [fetchData, query.refresh]);

  return (
    <Card className="grid grid-cols-1 md:grid-cols-5 gap-4 ">
      {localLoading ? (
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
              <div className="text-2xl font-bold">{data?.data?.uv}</div>
              <p
                className={`text-xs mt-1 flex items-center ${(data?.data?.uv_change ?? 0) >= 0 ? "text-red-500" : "text-green-500"}`}
              >
                <ChevronsUpDown className="h-3 w-3 mr-1" />
                {data?.data?.uv_change}%
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
                {data?.data?.pv}
              </div>
              <p
                className={`text-xs mt-1 flex items-center ${(data?.data?.pv_change ?? 0) >= 0 ? "text-red-500" : "text-green-500"}`}
              >
                <ChevronsUpDown className="h-3 w-3 mr-1" />
                {data?.data?.pv_change}%
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
                {data?.data?.bounce_rate}%
              </div>
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
                {data?.data?.avg_duration_format}
              </div>
              <p
                className={`text-xs mt-1 flex items-center ${(data?.data?.avg_duration_change ?? 0) >= 0 ? "text-red-500" : "text-green-500"}`}
              >
                <ChevronsUpDown className="h-3 w-3 mr-1" />
                {data?.data?.avg_duration_change}%
              </p>
            </CardContent>
          </div>

          <div className="col-span-1 md:col-span-1 p-4">
            <CardHeader className="p-0 pb-2">
              <CardTitle className="text-sm font-medium text-gray-500">
                单次访问浏览量
              </CardTitle>
            </CardHeader>
            <CardContent className="p-0 pt-0">
              <div className="text-2xl font-bold">
                {data?.data?.pv &&
                data?.data?.sessions &&
                data?.data?.sessions > 0
                  ? Math.round((data.data.pv / data?.data?.sessions) * 100) /
                    100
                  : "0"}
              </div>
            </CardContent>
          </div>
        </>
      )}
    </Card>
  );
}
