# Multi-stage build for React app
# Stage 1: Build the React app
FROM node:20-alpine AS builder

# Set working directory
WORKDIR /app

# Copy only package.json first
COPY lola-ia/package.json ./

# Configure npm to use public registry and remove any auth
RUN npm config set registry https://registry.npmjs.org/ && \
    npm config set //registry.npmjs.org/:_authToken "" && \
    npm config delete //registry.npmjs.org/:_authToken 2>/dev/null || true && \
    npm cache clean --force

# Install dependencies (this will create a fresh package-lock.json)
RUN npm install

# Copy source code
COPY lola-ia/ .

# Build the app
RUN npm run build

# Stage 2: Serve the app with nginx
FROM nginx:alpine

# Copy built app from builder stage
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy custom nginx config if needed (optional)
# COPY nginx.conf /etc/nginx/nginx.conf

# Expose port 80
EXPOSE 80

# Start nginx
CMD ["nginx", "-g", "daemon off;"]
