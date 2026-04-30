axios.defaults.headers.post['Content-Type'] = 'application/x-www-form-urlencoded; charset=UTF-8';
axios.defaults.headers.common['X-Requested-With'] = 'XMLHttpRequest';

const csrfMeta = document.querySelector('meta[name="csrf-token"]');
const basePathMeta = document.querySelector('meta[name="base-path"]');
const csrfToken = csrfMeta ? csrfMeta.getAttribute('content') : '';
const basePath = basePathMeta ? basePathMeta.getAttribute('content') : '/';

axios.defaults.baseURL = basePath;
if (csrfToken) {
    axios.defaults.headers.common['X-CSRF-Token'] = csrfToken;
}

axios.interceptors.request.use(
    config => {
        config.data = Qs.stringify(config.data, {
            arrayFormat: 'repeat'
        });
        return config;
    },
    error => Promise.reject(error)
);