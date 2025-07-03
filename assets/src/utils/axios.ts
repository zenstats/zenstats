import axios, { AxiosError, type AxiosInstance, type AxiosRequestConfig, type AxiosRequestHeaders, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios';
import useErrorStore from "@store/errorStore";
import qs from 'qs'

export interface BaseResponse<T = unknown> {
    code: number;
    message: string;
    error?: string;
    data: T;
}

const ignoreMsgs = [
    'invalid refresh token',
    'the refresh token has expired'
]

let requestList: Array<() => void> = []
let isRefreshToken = false
// 请求白名单，无须token的接口
const whiteList: string[] = ['/api/v1/token', '/login', '/v1/publicKey']
const api = axios.create({
    baseURL: 'http://localhost:8080/api',
}) as AxiosInstance & {
    <T = unknown>(config: AxiosRequestConfig): Promise<BaseResponse<T>>;
    <T = unknown>(url: string, config?: AxiosRequestConfig): Promise<BaseResponse<T>>;
};

// request拦截器
api.interceptors.request.use(
    (config: InternalAxiosRequestConfig) => {
        let isToken = true
        whiteList.some((v) => {
            if (config.url && config.url.startsWith(v)) {
                return isToken = false
            }
        })
        if (localStorage.getItem('token') && isToken) {
            config.headers.Authorization = 'Bearer ' + localStorage.getItem('token') // 让每个请求携带自定义token
        }

        const params = config.params || {}
        const data = config.data || false
        if (
            config.method?.toUpperCase() === 'POST' &&
            (config.headers as AxiosRequestHeaders)['Content-Type'] ===
            'application/x-www-form-urlencoded'
        ) {
            config.data = qs.stringify(data)
        }
        // get参数编码
        if (config.method?.toUpperCase() === 'GET' && params) {
            config.params = {}
            const paramsStr = qs.stringify(params, { allowDots: true })
            if (paramsStr) {
                config.url = config.url + '?' + paramsStr
            }
        }
        return config
    },
    (error: AxiosError) => {
        Promise.reject(error)
    }
)


// response 拦截器
api.interceptors.response.use(
    async (response: AxiosResponse<BaseResponse>) => {
        const { data: originalData, status } = response;
        const data = originalData;

        if (status === 500) {
            await handleError();
            return Promise.reject('服务器异常！');
        }

        if (!data) {
            throw new Error();
        }

        const code = data.code;
        const msg = data.message;
        if (ignoreMsgs.indexOf(msg) !== -1) {
            // 如果是忽略的错误码，直接返回 msg 异常
            return Promise.reject(msg);
        } else if (code === 430) {
            if (!isRefreshToken) {
                isRefreshToken = true;
                if (!localStorage.getItem("refreshToken")) {
                    await handleAuthorized();
                    return Promise.reject('认证失败');
                }
                try {
                    const refreshTokenRes = await refreshToken();
                    if (refreshTokenRes.data.code === 200) {
                        localStorage.setItem('token', refreshTokenRes.data.data);
                        const newConfig = { ...response.config };
                        newConfig.headers!.Authorization = 'Bearer ' + localStorage.getItem('token');
                        requestList.forEach((cb) => {
                            cb();
                        });
                        requestList = [];
                        return api(newConfig);
                    } else {
                        await handleAuthorized();
                        return Promise.reject('认证失败');
                    }
                } catch {
                    requestList.forEach((cb) => {
                        cb();
                    });
                    await handleAuthorized();
                    return Promise.reject('认证失败');
                } finally {
                    requestList = [];
                    isRefreshToken = false;
                }
            } else {
                // 添加到队列，等待刷新获取到新的令牌
                return new Promise((resolve) => {
                    requestList.push(() => {
                        const newConfig = { ...response.config };
                        newConfig.headers!.Authorization = 'Bearer ' + localStorage.getItem("token");
                        api(newConfig).then(resolve);
                    });
                });
            }
        } else if (code === 500) {
            await handleError();
            return Promise.reject('服务器异常');
        } else if (code === 403) {
            await handleForbidden();
            return Promise.reject('未授权访问');
        } else if (code === 406) {
            await handleAuthorized();
            return Promise.reject('认证失败');
        } else if (code === 401) {
            if (!window.location.href.includes('login')) {
                window.location.href = '/login';
            }
            localStorage.removeItem('token');
            localStorage.removeItem('refreshToken');
        }

        // 始终返回 AxiosResponse
        return response;
    },
    (error: AxiosError) => {
        // HTTP 错误处理
        const { setError } = useErrorStore.getState() as { setError: (msg: string) => void };
        const errorMessage =
            error.response?.data && typeof error.response.data === 'object' && 'message' in error.response.data
                ? (error.response.data as { message: string }).message
                : "网络错误";
        setError(errorMessage);

        return Promise.reject(error);
    }
);

const refreshToken = async () => {
    return await api.get('/auth/refresh?refreshToken=' + localStorage.getItem('refreshToken'))
}

const handleAuthorized = () => {
    // 如果已经到重新登录页面则不进行弹窗提示
    if (window.location.href.includes('login')) {
        return
    }
    localStorage.removeItem('token')
    localStorage.removeItem('refreshToken')
    window.location.href = '/login'
    return Promise.reject('认证失败')
}

const handleForbidden = () => {
    const { setError } = useErrorStore.getState() as { setError: (msg: string) => void };
    setError("未授权访问");
    return Promise.reject(new Error("未授权访问"));
}

const handleError = () => {
    const { setError } = useErrorStore.getState() as { setError: (msg: string) => void };
    setError("服务器异常");
    return Promise.reject(new Error("服务器异常"));
}

export default api;